// Package crypto provides cryptographic helpers for the Telegram MTProto protocol,
// including the SRP (Secure Remote Password) implementation used for 2FA.
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"math/big"

	"golang.org/x/crypto/pbkdf2"
)

// sizeForHash is the canonical byte-length used when padding integers for hashing.
// All 2048-bit prime and group-element values are padded to exactly this size.
const sizeForHash = 256

// SRPParams holds the parameters from Telegram's account.Password response.
type SRPParams struct {
	Salt1 []byte // KDF salt 1
	Salt2 []byte // KDF salt 2
	G     int    // Generator (typically 3)
	P     []byte // 2048-bit safe prime, big-endian
	SRPId int64  // srp_id from account.Password
	SRPB  []byte // g^b mod p from the server, big-endian, padded to 256 bytes
}

// SRPAnswer holds the (A, M1) pair to send in auth.checkPassword.
type SRPAnswer struct {
	A  []byte // g^a mod p, padded to 256 bytes
	M1 []byte // client proof of password knowledge (32 bytes)
}

// ComputeSRPAnswer computes the SRP answer for a Telegram 2FA password check.
// It implements the MTProto SRP protocol and is a direct port of
// Telethon's telethon/password.py::compute_check().
//
// The returned *SRPAnswer.A and *SRPAnswer.M1 are the values to place
// into tg.InputCheckPasswordSRP{SRPId, A, M1}.
func ComputeSRPAnswer(password string, params *SRPParams) (*SRPAnswer, error) {
	p := new(big.Int).SetBytes(params.P)
	g := big.NewInt(int64(params.G))
	B := new(big.Int).SetBytes(params.SRPB)

	// Validate that B is in the safe range (0 < B < p).
	if !isGoodLarge(B, p) {
		return nil, fmt.Errorf("srp: bad srp_B from server (out of range)")
	}

	// Precompute padded representations used in hashing.
	pForHash := numBytesForHash(params.P)
	gForHash := bigNumForHash(g)
	bForHash := numBytesForHash(params.SRPB)

	// x = PH2(password, salt1, salt2)
	pwHash := ph2([]byte(password), params.Salt1, params.Salt2)
	x := new(big.Int).SetBytes(pwHash)

	// k = H(p_padded, g_padded)
	k := new(big.Int).SetBytes(hashConcat(pForHash, gForHash))

	// g_x = g^x mod p
	gX := new(big.Int).Exp(g, x, p)

	// kg_x = k * g_x mod p
	kgX := new(big.Int).Mul(k, gX)
	kgX.Mod(kgX, p)

	// Generate a random 256-byte secret a; retry until A = g^a mod p is valid
	// and u = H(A_padded, B_padded) is non-zero.
	var aInt *big.Int
	var aForHash []byte
	var u *big.Int
	for {
		randBytes := make([]byte, 256)
		if _, err := rand.Read(randBytes); err != nil {
			return nil, fmt.Errorf("srp: crypto/rand: %w", err)
		}
		a := new(big.Int).SetBytes(randBytes)
		A := new(big.Int).Exp(g, a, p)
		if !isGoodModExpFirst(A, p) {
			continue
		}
		aFH := bigNumForHash(A)
		uVal := new(big.Int).SetBytes(hashConcat(aFH, bForHash))
		if uVal.Sign() <= 0 {
			continue
		}
		aInt = a
		aForHash = aFH
		u = uVal
		break
	}

	// g_b = (B - k*g_x) mod p
	gB := new(big.Int).Sub(B, kgX)
	gB.Mod(gB, p)
	if !isGoodModExpFirst(gB, p) {
		return nil, fmt.Errorf("srp: bad g_b value (out of range)")
	}

	// S = g_b ^ (a + u*x) mod p
	ux := new(big.Int).Mul(u, x)
	aUx := new(big.Int).Add(aInt, ux)
	S := new(big.Int).Exp(gB, aUx, p)

	// K = H(S padded to 256 bytes)
	K := hashConcat(bigNumForHash(S))

	// M1 = H(xor(H(p), H(g)), H(salt1), H(salt2), A, B, K)
	M1 := hashConcat(
		xorBytes(hashConcat(pForHash), hashConcat(gForHash)),
		hashConcat(params.Salt1),
		hashConcat(params.Salt2),
		aForHash,
		bForHash,
		K,
	)

	return &SRPAnswer{
		A:  aForHash,
		M1: M1,
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers (exported for testing via the crypto_test package)
// ---------------------------------------------------------------------------

// HashConcat computes SHA-256 of the concatenation of all inputs.
// Equivalent to Python's sha256(*p) in password.py.
func HashConcat(data ...[]byte) []byte { return hashConcat(data...) }

func hashConcat(data ...[]byte) []byte {
	h := sha256.New()
	for _, d := range data {
		h.Write(d)
	}
	return h.Sum(nil)
}

// SH computes SHA-256(salt || data || salt).
func SH(data, salt []byte) []byte { return sh(data, salt) }

func sh(data, salt []byte) []byte {
	return hashConcat(salt, data, salt)
}

// PH1 computes the first-stage password hash.
//
//	PH1(password, salt1, salt2) = SHA256(salt2 || SHA256(salt1 || password || salt1) || salt2)
func PH1(password, salt1, salt2 []byte) []byte { return ph1(password, salt1, salt2) }

func ph1(password, salt1, salt2 []byte) []byte {
	return sh(sh(password, salt1), salt2)
}

// PH2 computes the second-stage password hash using PBKDF2-HMAC-SHA512.
//
//	PH2(password, salt1, salt2) = SHA256(salt2 || PBKDF2(ph1(password,s1,s2), s1, 100000) || salt2)
func PH2(password, salt1, salt2 []byte) []byte { return ph2(password, salt1, salt2) }

func ph2(password, salt1, salt2 []byte) []byte {
	interim := ph1(password, salt1, salt2)
	hash3 := pbkdf2SHA512(interim, salt1, 100000)
	return sh(hash3, salt2)
}

// Pbkdf2SHA512 derives a 64-byte key using PBKDF2-HMAC-SHA512.
func Pbkdf2SHA512(password, salt []byte, iterations int) []byte {
	return pbkdf2SHA512(password, salt, iterations)
}

func pbkdf2SHA512(password, salt []byte, iterations int) []byte {
	return pbkdf2.Key(password, salt, iterations, 64, sha512.New)
}

// NumBytesForHash left-pads b with zero bytes until the result is exactly
// sizeForHash (256) bytes long.
func NumBytesForHash(b []byte) []byte { return numBytesForHash(b) }

func numBytesForHash(b []byte) []byte {
	if len(b) >= sizeForHash {
		return b
	}
	padded := make([]byte, sizeForHash)
	copy(padded[sizeForHash-len(b):], b)
	return padded
}

// BigNumForHash encodes n as a sizeForHash-byte (256-byte) big-endian slice.
func BigNumForHash(n *big.Int) []byte { return bigNumForHash(n) }

func bigNumForHash(n *big.Int) []byte {
	return numBytesForHash(n.Bytes())
}

// XorBytes XORs two equally-sized byte slices element-wise.
func XorBytes(a, b []byte) []byte { return xorBytes(a, b) }

func xorBytes(a, b []byte) []byte {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = a[i] ^ b[i]
	}
	return out
}

// isGoodLarge checks number > 0 && p−number > 0.
func isGoodLarge(number, p *big.Int) bool {
	if number.Sign() <= 0 {
		return false
	}
	diff := new(big.Int).Sub(p, number)
	return diff.Sign() > 0
}

// isGoodModExpFirst checks that the modular exponentiation result is safe to use.
// Mirrors the Python is_good_mod_exp_first() in password.py.
func isGoodModExpFirst(modexp, prime *big.Int) bool {
	diff := new(big.Int).Sub(prime, modexp)
	const minDiffBits = 2048 - 64
	const maxModExpSize = 256

	if diff.Sign() < 0 {
		return false
	}
	if diff.BitLen() < minDiffBits {
		return false
	}
	if modexp.BitLen() < minDiffBits {
		return false
	}
	if (modexp.BitLen()+7)/8 > maxModExpSize {
		return false
	}
	return true
}
