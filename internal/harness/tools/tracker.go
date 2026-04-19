package tools

import (
	"path/filepath"
	"sync"
)

// FileTracker records which files have been Read in the current
// session. Edit / Write tools enforce a "Read before mutate" contract
// to make the model think before clobbering.
type FileTracker struct {
	mu   sync.Mutex
	read map[string]bool // absolute path → has been read
}

// NewFileTracker returns an empty tracker.
func NewFileTracker() *FileTracker {
	return &FileTracker{read: make(map[string]bool)}
}

// MarkRead records that a file has been read. Path is canonicalized
// to its absolute form for consistent lookups.
func (ft *FileTracker) MarkRead(path string) {
	canon := canonicalize(path)
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.read[canon] = true
}

// HasRead reports whether the file has been read in this session.
func (ft *FileTracker) HasRead(path string) bool {
	canon := canonicalize(path)
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.read[canon]
}

// Reset clears all tracked reads. Used on session.clear.
func (ft *FileTracker) Reset() {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.read = make(map[string]bool)
}

func canonicalize(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return filepath.Clean(abs)
}
