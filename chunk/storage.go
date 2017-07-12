package chunk

import (
	"errors"
	"fmt"
	"os"
	"sync"

	. "github.com/claudetech/loggo/default"
)

// ErrTimeout is a timeout error
var ErrTimeout = errors.New("timeout")

// Storage is a chunk storage
type Storage struct {
	ChunkPath string
	ChunkSize int64
	MaxChunks int
	chunks    map[string][]byte
	stack     []string
	lock      sync.RWMutex
}

// Item represents a chunk in RAM
type Item struct {
	id    string
	bytes []byte
}

// NewStorage creates a new storage
func NewStorage(chunkPath string, chunkSize int64, maxChunks int) *Storage {
	storage := Storage{
		ChunkPath: chunkPath,
		ChunkSize: chunkSize,
		MaxChunks: maxChunks,
		chunks:    make(map[string][]byte),
		stack:     make([]string, maxChunks),
	}

	return &storage
}

// Clear removes all old chunks on disk (will be called on each program start)
func (s *Storage) Clear() error {
	if err := os.RemoveAll(s.ChunkPath); nil != err {
		return fmt.Errorf("Could not clear old chunks from disk")
	}
	return nil
}

// LoadOrCreate loads a chunk from ram or creates it
func (s *Storage) LoadOrCreate(id string) ([]byte, bool) {
	s.lock.Lock()
	if chunk, exists := s.chunks[id]; exists {
		s.lock.Unlock()
		return chunk, true
	}
	s.chunks[id] = nil
	s.lock.Unlock()
	return nil, false
}

// Store stores a chunk in the RAM and adds it to the disk storage queue
func (s *Storage) Store(id string, bytes []byte) error {
	s.lock.Lock()
	s.stack = append(s.stack, id)
	if len(s.stack) > s.MaxChunks {
		deleteID := s.stack[0]
		if "" != deleteID {
			s.stack = s.stack[1:]
			Log.Debugf("Deleting chunk %v", deleteID)
			delete(s.chunks, deleteID)
		}
	}
	s.chunks[id] = bytes
	s.lock.Unlock()

	return nil
}

// Error is called to remove an item from the index if there has been an issue downloading the chunk
func (s *Storage) Error(id string) {
	s.lock.Lock()
	delete(s.chunks, id)
	s.lock.Unlock()
}
