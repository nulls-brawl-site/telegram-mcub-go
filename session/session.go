// Package session provides session management for telegram-mcub-go.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
