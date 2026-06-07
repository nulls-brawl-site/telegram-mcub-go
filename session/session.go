// Package session provides session management for telegram-mcub-go.
package session

import (
	"context"
	"crypto/sha1" // #nosec
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/gotd/td/session"
)

// FileSessionStorage wraps gotd's file-based session storage with MCUB extensions.
type FileSessionStorage struct {
	mu      sync.RWMutex
	path    string
	storage *session.FileStorage
}

// NewFileSessionStorage creates a new file-based session storage at the given path.
func NewFileSessionStorage(path string) (*FileSessionStorage, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	return &FileSessionStorage{
		path:    path,
		storage: &session.FileStorage{Path: path},
	}, nil
}

// Storage returns the underlying gotd session.Storage implementation.
func (f *FileSessionStorage) Storage() session.Storage {
	return f.storage
}

// Path returns the session file path.
func (f *FileSessionStorage) Path() string {
	return f.path
}

// ResumeState stores the progress of a resumable transfer (download or upload).
type ResumeState struct {
	// ID is the operation identifier (e.g. file path or custom key).
	ID string `json:"id"`
	// Kind is "download" or "upload".
	Kind string `json:"kind"`

	BytesDone  int64  `json:"bytes_done"`
	TotalBytes int64  `json:"total_bytes"`
	PartsDone  int    `json:"parts_done"`
	TotalParts int    `json:"total_parts"`
	FileID     int64  `json:"file_id,omitempty"`
	AccessHash int64  `json:"access_hash,omitempty"`
	DCID       int    `json:"dc_id,omitempty"`
	Completed  bool   `json:"completed"`
	DestPath   string `json:"dest_path,omitempty"`
}

// StateStore persists and retrieves ResumeState values on disk.
type StateStore struct {
	mu  sync.RWMutex
	dir string
}

// NewStateStore creates a StateStore backed by the given directory.
func NewStateStore(dir string) (*StateStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create state store dir: %w", err)
	}
	return &StateStore{dir: dir}, nil
}

func (s *StateStore) keyPath(id, kind string) string {
	safe := filepath.Base(id)
	if len(safe) > 64 {
		safe = safe[:64]
	}
	return filepath.Join(s.dir, fmt.Sprintf("%s_%s.json", kind, safe))
}

// Save writes the state to disk. ID and Kind must be set.
func (s *StateStore) Save(state *ResumeState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.keyPath(state.ID, state.Kind)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// Load retrieves a previously saved state. Returns nil, nil if not found.
func (s *StateStore) Load(id, kind string) (*ResumeState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.keyPath(id, kind)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	var state ResumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}
	return &state, nil
}

// Delete removes a saved state from disk.
func (s *StateStore) Delete(id, kind string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.keyPath(id, kind)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete state: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// MemorySessionStorage — ported from Telethon-MCUB; useful for testing.
// ─────────────────────────────────────────────────────────────────────────────

// MemorySessionStorage stores a gotd session entirely in memory.
// It is lost when the process exits — use it for tests or ephemeral sessions.
type MemorySessionStorage struct {
	mu   sync.RWMutex
	data []byte
}

// NewMemorySessionStorage creates an empty in-memory session storage.
func NewMemorySessionStorage() *MemorySessionStorage {
	return &MemorySessionStorage{}
}

// Storage returns the MemorySessionStorage itself as a session.Storage.
func (m *MemorySessionStorage) Storage() session.Storage {
	return m
}

// LoadSession implements session.Storage.
func (m *MemorySessionStorage) LoadSession(_ context.Context) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.data) == 0 {
		return nil, nil
	}
	out := make([]byte, len(m.data))
	copy(out, m.data)
	return out, nil
}

// StoreSession implements session.Storage.
func (m *MemorySessionStorage) StoreSession(_ context.Context, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make([]byte, len(data))
	copy(m.data, data)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// SQLiteSessionStorage — persists the session in a SQLite database.
// The caller must blank-import a SQLite3 driver, e.g.:
//
//	import _ "github.com/mattn/go-sqlite3"
//
// ─────────────────────────────────────────────────────────────────────────────

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS mcub_session (
    id   INTEGER PRIMARY KEY,
    data BLOB    NOT NULL
);
`

const sqliteKey = 1

// SQLiteSessionStorage persists a gotd session in a SQLite database.
type SQLiteSessionStorage struct {
	path string
	db   *sql.DB
}

// NewSQLiteSessionStorage opens (or creates) the SQLite database at path and
// initialises the session table.  The caller must have imported a SQLite driver
// (e.g. github.com/mattn/go-sqlite3) before calling this function.
func NewSQLiteSessionStorage(path string) (*SQLiteSessionStorage, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite3: %w", err)
	}

	if _, err := db.Exec(sqliteSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return &SQLiteSessionStorage{path: path, db: db}, nil
}

// Storage returns the SQLiteSessionStorage itself as a session.Storage.
func (s *SQLiteSessionStorage) Storage() session.Storage {
	return s
}

// Close closes the underlying database connection.
func (s *SQLiteSessionStorage) Close() error {
	return s.db.Close()
}

// LoadSession implements session.Storage.
func (s *SQLiteSessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT data FROM mcub_session WHERE id = ?`, sqliteKey,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	return data, nil
}

// StoreSession implements session.Storage.
func (s *SQLiteSessionStorage) StoreSession(ctx context.Context, data []byte) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mcub_session (id, data) VALUES (?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data`,
		sqliteKey, data,
	)
	if err != nil {
		return fmt.Errorf("store session: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// StringSession — Telethon-compatible string session codec.
//
// Binary layout (big-endian, no version byte in binary payload):
//
//	┌────────┬──────────────┬──────────┬─────────────────┐
//	│ DC ID  │ IP address   │ Port     │ Auth key         │
//	│ 1 byte │ 4 or 16 bytes│ 2 bytes  │ 256 bytes        │
//	└────────┴──────────────┴──────────┴─────────────────┘
//	= 263 bytes (IPv4) or 275 bytes (IPv6)
//
// The binary is base64url-encoded (with standard '=' padding, matching Python's
// base64.urlsafe_b64encode), then prefixed with the ASCII character "1".
//
// Reference: Telethon sessions/string.py
// ─────────────────────────────────────────────────────────────────────────────

const stringSessionVersion = '1'

// StringSession encodes/decodes Telethon-compatible session strings.
type StringSession struct {
	DCID       int
	ServerAddr string // plain IPv4 or IPv6, e.g. "149.154.167.51"
	Port       int
	AuthKey    []byte // exactly 256 bytes when set
}

// NewStringSession creates an empty StringSession.
func NewStringSession() *StringSession {
	return &StringSession{}
}

// ParseString parses a Telethon string session.
// The string must start with "1" (the current Telethon session version).
// An empty string returns an empty session without error.
func ParseString(s string) (*StringSession, error) {
	if s == "" {
		return NewStringSession(), nil
	}
	if s[0] != stringSessionVersion {
		return nil, fmt.Errorf("unsupported string session version %q (expected '1')", s[0])
	}

	// Try standard base64url (with '=' padding) first, then raw (without padding).
	raw, err := base64.URLEncoding.DecodeString(s[1:])
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(s[1:])
		if err != nil {
			return nil, fmt.Errorf("decode string session: %w", err)
		}
	}

	var ipLen int
	switch len(raw) {
	case 263:
		ipLen = 4
	case 275:
		ipLen = 16
	default:
		return nil, fmt.Errorf("invalid string session payload length: %d (want 263 or 275)", len(raw))
	}

	dcID := int(raw[0])
	ip := net.IP(raw[1 : 1+ipLen])
	port := int(binary.BigEndian.Uint16(raw[1+ipLen : 3+ipLen]))
	authKey := make([]byte, 256)
	copy(authKey, raw[3+ipLen:])

	return &StringSession{
		DCID:       dcID,
		ServerAddr: ip.String(),
		Port:       port,
		AuthKey:    authKey,
	}, nil
}

// Encode encodes the session to a Telethon-compatible string.
// Returns "" if AuthKey is not set.
func (s *StringSession) Encode() string {
	if len(s.AuthKey) == 0 {
		return ""
	}

	ip := net.ParseIP(s.ServerAddr)
	if ip == nil {
		return ""
	}

	var ipBytes []byte
	if ip4 := ip.To4(); ip4 != nil {
		ipBytes = []byte(ip4)
	} else if ip6 := ip.To16(); ip6 != nil {
		ipBytes = []byte(ip6)
	} else {
		return ""
	}

	// Pack: dc_id(1) + ip(4|16) + port(2) + auth_key(256)
	buf := make([]byte, 1+len(ipBytes)+2+256)
	buf[0] = byte(s.DCID)
	copy(buf[1:], ipBytes)
	binary.BigEndian.PutUint16(buf[1+len(ipBytes):], uint16(s.Port))
	key := make([]byte, 256)
	copy(key, s.AuthKey)
	copy(buf[3+len(ipBytes):], key)

	return string([]byte{stringSessionVersion}) + base64.URLEncoding.EncodeToString(buf)
}

// ToStorage converts the StringSession into a gotd-compatible session.Storage.
// The returned storage is backed by memory and can be passed directly to
// telegram.Options.SessionStorage. Changes written back via StoreSession will
// update the in-memory blob (but not this StringSession struct).
func (s *StringSession) ToStorage() session.Storage {
	mem := NewMemorySessionStorage()

	if len(s.AuthKey) == 0 {
		return mem
	}

	// Compute auth_key_id: last 8 bytes of SHA1(auth_key).
	h := sha1.Sum(s.AuthKey) // #nosec
	authKeyID := h[12:]      // bytes 12..19

	// Build gotd session JSON: {"Version":1,"Data":{...}}
	data := gotdSessionData{
		Version: 1,
		Data: gotdDataInner{
			DC:        s.DCID,
			Addr:      net.JoinHostPort(s.ServerAddr, strconv.Itoa(s.Port)),
			AuthKey:   s.AuthKey,
			AuthKeyID: authKeyID,
			Salt:      0,
		},
	}

	blob, err := json.Marshal(data)
	if err != nil {
		return mem
	}

	_ = mem.StoreSession(context.Background(), blob)
	return mem
}

// gotdSessionData mirrors the private jsonData layout used by gotd's session.Loader.
type gotdSessionData struct {
	Version int           `json:"Version"`
	Data    gotdDataInner `json:"Data"`
}

// gotdDataInner mirrors session.Data in gotd (public field names, standard JSON tags).
type gotdDataInner struct {
	Config    gotdConfig `json:"Config"`
	DC        int        `json:"DC"`
	Addr      string     `json:"Addr"`
	AuthKey   []byte     `json:"AuthKey"`
	AuthKeyID []byte     `json:"AuthKeyID"`
	Salt      int64      `json:"Salt"`
}

// gotdConfig mirrors the subset of tg.Config stored in gotd sessions.
// We only need to supply enough fields for the JSON schema to unmarshal cleanly.
type gotdConfig struct {
	BlockedMode     bool        `json:"BlockedMode"`
	ForceTryIpv6    bool        `json:"ForceTryIpv6"`
	Date            int         `json:"Date"`
	Expires         int         `json:"Expires"`
	TestMode        bool        `json:"TestMode"`
	ThisDC          int         `json:"ThisDC"`
	DCOptions       interface{} `json:"DCOptions"`
	DCTxtDomainName string      `json:"DCTxtDomainName"`
	TmpSessions     int         `json:"TmpSessions"`
	WebfileDCID     int         `json:"WebfileDCID"`
}
