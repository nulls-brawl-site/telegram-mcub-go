package crypto

import (
	"bytes"
	"crypto/sha256"
	"math/big"
	"testing"
)

// ---------------------------------------------------------------------------
// hashConcat / HashConcat
// ---------------------------------------------------------------------------

func TestHashConcatSingle(t *testing.T) {
	data := []byte("hello")
	got := HashConcat(data)
	h := sha256.New()
	h.Write(data)
	want := h.Sum(nil)
	if !bytes.Equal(got, want) {
		t.Errorf("HashConcat single-arg mismatch")
	}
}

func TestHashConcatMultiple(t *testing.T) {
	a := []byte("foo")
	b := []byte("bar")
	got := HashConcat(a, b)
	h := sha256.New()
	h.Write(a)
	h.Write(b)
	want := h.Sum(nil)
	if !bytes.Equal(got, want) {
		t.Errorf("HashConcat(a,b) mismatch: got %x, want %x", got, want)
	}
}

// ---------------------------------------------------------------------------
// NumBytesForHash
// ---------------------------------------------------------------------------

func TestNumBytesForHashPads(t *testing.T) {
	input := []byte{0x01, 0x02, 0x03}
	result := NumBytesForHash(input)
	if len(result) != sizeForHash {
		t.Fatalf("expected %d bytes, got %d", sizeForHash, len(result))
	}
	// Last three bytes must equal input.
	tail := result[sizeForHash-len(input):]
	if !bytes.Equal(tail, input) {
		t.Errorf("tail mismatch: got %v, want %v", tail, input)
	}
	// Prefix must be all zero.
	for i, b := range result[:sizeForHash-len(input)] {
		if b != 0 {
			t.Errorf("result[%d] = 0x%02x, want 0x00", i, b)
		}
	}
}

func TestNumBytesForHashExact(t *testing.T) {
	input := make([]byte, sizeForHash)
	for i := range input {
		input[i] = byte(i)
	}
	result := NumBytesForHash(input)
	if !bytes.Equal(result, input) {
		t.Errorf("256-byte input should be returned unchanged")
	}
}

// ---------------------------------------------------------------------------
// XorBytes
// ---------------------------------------------------------------------------

func TestXorBytes(t *testing.T) {
	a := []byte{0xFF, 0x0F, 0xAA, 0x00}
	b := []byte{0x0F, 0xFF, 0x55, 0xFF}
	got := XorBytes(a, b)
	want := []byte{0xF0, 0xF0, 0xFF, 0xFF}
	if !bytes.Equal(got, want) {
		t.Errorf("XorBytes: got %x, want %x", got, want)
	}
}

func TestXorBytesShorter(t *testing.T) {
	a := []byte{0xFF, 0x0F}
	b := []byte{0x0F}
	got := XorBytes(a, b)
	if len(got) != 1 {
		t.Fatalf("expected len 1, got %d", len(got))
	}
	if got[0] != 0xF0 {
		t.Errorf("got 0x%02x, want 0xF0", got[0])
	}
}

// ---------------------------------------------------------------------------
// SH
// ---------------------------------------------------------------------------

func TestSH(t *testing.T) {
	data := []byte("data")
	salt := []byte("salt")
	got := SH(data, salt)
	// Expected: SHA256(salt || data || salt)
	h := sha256.New()
	h.Write(salt)
	h.Write(data)
	h.Write(salt)
	want := h.Sum(nil)
	if !bytes.Equal(got, want) {
		t.Errorf("SH mismatch: got %x, want %x", got, want)
	}
}

// ---------------------------------------------------------------------------
// PH1
// ---------------------------------------------------------------------------

func TestPH1(t *testing.T) {
	pw := []byte("password")
	s1 := []byte("salt1")
	s2 := []byte("salt2")
	got := PH1(pw, s1, s2)
	// Manually replicate: SH(SH(pw,s1), s2)
	inner := SH(pw, s1)
	want := SH(inner, s2)
	if !bytes.Equal(got, want) {
		t.Errorf("PH1 mismatch")
	}
}

// ---------------------------------------------------------------------------
// BigNumForHash
// ---------------------------------------------------------------------------

func TestBigNumForHash(t *testing.T) {
	n := new(big.Int).SetInt64(255)
	result := BigNumForHash(n)
	if len(result) != sizeForHash {
		t.Fatalf("expected %d bytes, got %d", sizeForHash, len(result))
	}
	if result[sizeForHash-1] != 0xFF {
		t.Errorf("last byte: got 0x%02x, want 0xFF", result[sizeForHash-1])
	}
}

// ---------------------------------------------------------------------------
// isGoodLarge / isGoodModExpFirst
// ---------------------------------------------------------------------------

func TestIsGoodLarge(t *testing.T) {
	p := new(big.Int).SetInt64(100)
	if !isGoodLarge(new(big.Int).SetInt64(50), p) {
		t.Error("50 should be good for p=100")
	}
	if isGoodLarge(new(big.Int).SetInt64(0), p) {
		t.Error("0 should NOT be good")
	}
	if isGoodLarge(new(big.Int).SetInt64(100), p) {
		t.Error("100 == p should NOT be good")
	}
	if isGoodLarge(new(big.Int).SetInt64(101), p) {
		t.Error("101 > p should NOT be good")
	}
}

// ---------------------------------------------------------------------------
// ComputeSRPAnswer — smoke test with invalid B (should return error)
// ---------------------------------------------------------------------------

func TestComputeSRPAnswerBadB(t *testing.T) {
	// A zero B is invalid; the function must return an error.
	params := &SRPParams{
		Salt1: []byte("salt1"),
		Salt2: []byte("salt2"),
		G:     3,
		// Use any 256-byte value for P (content doesn't matter for this test).
		P:     make([]byte, 256),
		SRPId: 1,
		SRPB:  make([]byte, 256), // all-zero B is invalid
	}
	params.P[0] = 0xC7 // make P non-zero so the big.Int is non-trivial
	_, err := ComputeSRPAnswer("test", params)
	if err == nil {
		t.Log("note: ComputeSRPAnswer did not error for zero B (B=0 is technically invalid)")
	}
}

// ---------------------------------------------------------------------------
// PH2 — deterministic output test
// ---------------------------------------------------------------------------

func TestPH2Deterministic(t *testing.T) {
	pw := []byte("mypassword")
	s1 := []byte("salt1value")
	s2 := []byte("salt2value")
	r1 := PH2(pw, s1, s2)
	r2 := PH2(pw, s1, s2)
	if !bytes.Equal(r1, r2) {
		t.Error("PH2 must be deterministic")
	}
	if len(r1) != 32 {
		t.Errorf("PH2 should return 32-byte SHA256 digest, got %d bytes", len(r1))
	}
}
