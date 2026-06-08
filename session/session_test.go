package session_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/nulls-brawl-site/telegram-mcub-go/session"
)

// ─────────────────────────────────────────────────────────────────────────────
// StringSession Encode / ParseString
// ─────────────────────────────────────────────────────────────────────────────

func makeAuthKey() []byte {
	key := make([]byte, 256)
	for i := range key {
		key[i] = byte(i % 256)
	}
	return key
}

func TestStringSessionEncodeNotEmpty(t *testing.T) {
	s := session.NewStringSession()
	s.DCID = 2
	s.ServerAddr = "149.154.167.51"
	s.Port = 443
	s.AuthKey = makeAuthKey()

	encoded := s.Encode()
	if encoded == "" {
		t.Fatal("Encode() returned empty string")
	}
	if encoded[0] != '1' {
		t.Errorf("first char: got %c, want '1'", encoded[0])
	}
}

func TestStringSessionEncodeEmptyAuthKey(t *testing.T) {
	s := session.NewStringSession()
	s.DCID = 2
	s.ServerAddr = "149.154.167.51"
	s.Port = 443
	// No AuthKey set → should return empty.
	if enc := s.Encode(); enc != "" {
		t.Errorf("Encode() without AuthKey: got %q, want empty", enc)
	}
}

func TestStringSessionEncodeInvalidAddr(t *testing.T) {
	s := session.NewStringSession()
	s.DCID = 1
	s.ServerAddr = "not-an-ip"
	s.Port = 443
	s.AuthKey = makeAuthKey()
	if enc := s.Encode(); enc != "" {
		t.Errorf("Encode() with invalid addr: got %q, want empty", enc)
	}
}

func TestStringSessionRoundTripIPv4(t *testing.T) {
	orig := &session.StringSession{
		DCID:       2,
		ServerAddr: "149.154.167.51",
		Port:       443,
		AuthKey:    makeAuthKey(),
	}

	encoded := orig.Encode()
	if encoded == "" {
		t.Fatal("Encode() returned empty string")
	}

	parsed, err := session.ParseString(encoded)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}

	if parsed.DCID != orig.DCID {
		t.Errorf("DCID: got %d, want %d", parsed.DCID, orig.DCID)
	}
	if parsed.ServerAddr != orig.ServerAddr {
		t.Errorf("ServerAddr: got %q, want %q", parsed.ServerAddr, orig.ServerAddr)
	}
	if parsed.Port != orig.Port {
		t.Errorf("Port: got %d, want %d", parsed.Port, orig.Port)
	}
	if !bytes.Equal(parsed.AuthKey, orig.AuthKey) {
		t.Error("AuthKey mismatch after round-trip")
	}
}

func TestStringSessionRoundTripAllDCs(t *testing.T) {
	addrs := []struct {
		dcid int
		addr string
		port int
	}{
		{1, "149.154.175.53", 443},
		{2, "149.154.167.51", 443},
		{3, "149.154.175.100", 443},
		{4, "149.154.167.91", 443},
		{5, "91.108.56.130", 443},
	}
	for _, dc := range addrs {
		s := &session.StringSession{
			DCID:       dc.dcid,
			ServerAddr: dc.addr,
			Port:       dc.port,
			AuthKey:    makeAuthKey(),
		}
		enc := s.Encode()
		parsed, err := session.ParseString(enc)
		if err != nil {
			t.Errorf("DC%d ParseString: %v", dc.dcid, err)
			continue
		}
		if parsed.DCID != dc.dcid {
			t.Errorf("DC%d DCID mismatch: got %d", dc.dcid, parsed.DCID)
		}
		if parsed.ServerAddr != dc.addr {
			t.Errorf("DC%d ServerAddr mismatch: got %q", dc.dcid, parsed.ServerAddr)
		}
	}
}

func TestParseStringEmpty(t *testing.T) {
	s, err := session.ParseString("")
	if err != nil {
		t.Fatalf("ParseString empty: %v", err)
	}
	if s == nil {
		t.Fatal("got nil session for empty string")
	}
}

func TestParseStringWrongVersion(t *testing.T) {
	_, err := session.ParseString("2AAAA")
	if err == nil {
		t.Error("expected error for wrong version byte")
	}
}

func TestParseStringInvalidBase64(t *testing.T) {
	_, err := session.ParseString("1!!!invalid!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestParseStringWrongLength(t *testing.T) {
	// Valid base64 but wrong payload length.
	import64 := "1" + "AAAA" // too short after decode
	_, err := session.ParseString(import64)
	if err == nil {
		t.Error("expected error for wrong payload length")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MemorySessionStorage
// ─────────────────────────────────────────────────────────────────────────────

func TestMemorySessionStorageEmpty(t *testing.T) {
	m := session.NewMemorySessionStorage()
	data, err := m.LoadSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if data != nil {
		t.Errorf("expected nil data for empty storage, got %v", data)
	}
}

func TestMemorySessionStorageRoundTrip(t *testing.T) {
	m := session.NewMemorySessionStorage()
	blob := []byte(`{"version":1,"data":"test"}`)

	if err := m.StoreSession(context.Background(), blob); err != nil {
		t.Fatalf("StoreSession: %v", err)
	}

	loaded, err := m.LoadSession(context.Background())
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if !bytes.Equal(loaded, blob) {
		t.Errorf("loaded data mismatch: got %q, want %q", loaded, blob)
	}
}

func TestMemorySessionStorageOverwrite(t *testing.T) {
	m := session.NewMemorySessionStorage()
	first := []byte("first")
	second := []byte("second")

	_ = m.StoreSession(context.Background(), first)
	_ = m.StoreSession(context.Background(), second)

	loaded, _ := m.LoadSession(context.Background())
	if !bytes.Equal(loaded, second) {
		t.Errorf("expected overwritten value %q, got %q", second, loaded)
	}
}

func TestMemorySessionStorageIsolation(t *testing.T) {
	// Modifying the returned slice must not affect stored data.
	m := session.NewMemorySessionStorage()
	blob := []byte("data")
	_ = m.StoreSession(context.Background(), blob)

	loaded1, _ := m.LoadSession(context.Background())
	loaded1[0] = 'X' // mutate

	loaded2, _ := m.LoadSession(context.Background())
	if loaded2[0] != 'd' {
		t.Error("mutation of returned slice affected stored data")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CachedSessionStorage
// ─────────────────────────────────────────────────────────────────────────────

func TestCachedSessionStorageRoundTrip(t *testing.T) {
	inner := session.NewMemorySessionStorage()
	cached := session.NewCachedSessionStorage(inner)

	blob := []byte("session-data")
	if err := cached.StoreSession(context.Background(), blob); err != nil {
		t.Fatalf("StoreSession: %v", err)
	}

	loaded, err := cached.LoadSession(context.Background())
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if !bytes.Equal(loaded, blob) {
		t.Errorf("loaded data mismatch: got %q, want %q", loaded, blob)
	}
}

func TestCachedSessionStorageInvalidate(t *testing.T) {
	inner := session.NewMemorySessionStorage()
	cached := session.NewCachedSessionStorage(inner)

	blob := []byte("initial")
	_ = cached.StoreSession(context.Background(), blob)

	// Invalidate should force re-read from inner.
	newBlob := []byte("updated")
	_ = inner.StoreSession(context.Background(), newBlob)
	cached.Invalidate()

	loaded, _ := cached.LoadSession(context.Background())
	if !bytes.Equal(loaded, newBlob) {
		t.Errorf("after invalidate: got %q, want %q", loaded, newBlob)
	}
}

func TestCachedSessionStoragePanicsOnNilInner(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil inner storage")
		}
	}()
	session.NewCachedSessionStorage(nil)
}

// ─────────────────────────────────────────────────────────────────────────────
// StateStore
// ─────────────────────────────────────────────────────────────────────────────

func TestStateStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewStateStore(dir)
	if err != nil {
		t.Fatalf("NewStateStore: %v", err)
	}

	state := &session.ResumeState{
		ID:         "test-file.mp4",
		Kind:       "download",
		BytesDone:  1024,
		TotalBytes: 4096,
		PartsDone:  2,
		TotalParts: 8,
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("test-file.mp4", "download")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.BytesDone != state.BytesDone {
		t.Errorf("BytesDone: got %d, want %d", loaded.BytesDone, state.BytesDone)
	}
	if loaded.TotalBytes != state.TotalBytes {
		t.Errorf("TotalBytes: got %d, want %d", loaded.TotalBytes, state.TotalBytes)
	}
}

func TestStateStoreLoadMissing(t *testing.T) {
	dir := t.TempDir()
	store, _ := session.NewStateStore(dir)

	s, err := store.Load("nonexistent", "download")
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if s != nil {
		t.Error("expected nil for missing state")
	}
}

func TestStateStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, _ := session.NewStateStore(dir)

	state := &session.ResumeState{ID: "x", Kind: "upload", TotalBytes: 100}
	_ = store.Save(state)
	_ = store.Delete("x", "upload")

	s, err := store.Load("x", "upload")
	if err != nil {
		t.Fatalf("Load after delete: %v", err)
	}
	if s != nil {
		t.Error("expected nil after delete")
	}
}

func TestStateStoreDeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, _ := session.NewStateStore(dir)
	// Deleting a nonexistent entry should not error.
	if err := store.Delete("ghost", "download"); err != nil {
		t.Errorf("Delete nonexistent: %v", err)
	}
}
