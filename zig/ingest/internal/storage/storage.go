package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Store manages bundle files on the local filesystem.
type Store struct {
	baseDir string
	mu      sync.Mutex
	hashers map[string]hash.Hash // keyed by upload ID
}

// New creates a Store rooted at baseDir, creating the directory if needed.
func New(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create bundle dir: %w", err)
	}
	return &Store{
		baseDir: baseDir,
		hashers: make(map[string]hash.Hash),
	}, nil
}

// BundlePath returns the absolute path for a given upload ID.
func (s *Store) BundlePath(uploadID string) string {
	// Organize into two-character prefix directories to avoid large flat dirs.
	prefix := uploadID
	if len(prefix) >= 2 {
		prefix = prefix[:2]
	}
	return filepath.Join(s.baseDir, prefix, uploadID+".bundle")
}

// InitFile creates the directory and file for a new upload. Returns the storage path.
func (s *Store) InitFile(uploadID string) (string, error) {
	path := s.BundlePath(uploadID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create bundle subdir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create bundle file: %w", err)
	}
	f.Close()

	s.mu.Lock()
	s.hashers[uploadID] = sha256.New()
	s.mu.Unlock()

	return path, nil
}

// WriteChunk appends data at the given offset. It also feeds the data into
// the running SHA-256 hash.
//
// The caller must supply chunks in order — random-access writes are not
// supported because the SHA-256 must be computed sequentially.
func (s *Store) WriteChunk(uploadID string, offset int64, data []byte) (newOffset int64, err error) {
	path := s.BundlePath(uploadID)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open bundle for write: %w", err)
	}
	defer f.Close()

	n, err := f.WriteAt(data, offset)
	if err != nil {
		return 0, fmt.Errorf("write chunk at offset %d: %w", offset, err)
	}

	s.mu.Lock()
	h, ok := s.hashers[uploadID]
	if ok {
		h.Write(data[:n])
	}
	s.mu.Unlock()

	return offset + int64(n), nil
}

// FinalHash returns the hex-encoded SHA-256 of everything written so far and
// removes the hasher from memory.
func (s *Store) FinalHash(uploadID string) (string, error) {
	s.mu.Lock()
	h, ok := s.hashers[uploadID]
	if ok {
		delete(s.hashers, uploadID)
	}
	s.mu.Unlock()

	if !ok {
		// Fall back to hashing the file on disk.
		return s.hashFile(uploadID)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashFile computes SHA-256 by reading the entire file from disk.
func (s *Store) hashFile(uploadID string) (string, error) {
	path := s.BundlePath(uploadID)
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open bundle for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash bundle: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileSize returns the current size of a bundle file.
func (s *Store) FileSize(uploadID string) (int64, error) {
	path := s.BundlePath(uploadID)
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ResumeHash rebuilds the running SHA-256 from the existing file content so
// that a resumed upload can continue computing the hash correctly.
func (s *Store) ResumeHash(uploadID string) error {
	path := s.BundlePath(uploadID)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open bundle for resume hash: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("resume hash: %w", err)
	}

	s.mu.Lock()
	s.hashers[uploadID] = h
	s.mu.Unlock()

	return nil
}
