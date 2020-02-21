package chunk

import (
	"errors"
	"sync"

	. "github.com/claudetech/loggo/default"
)

// ErrTimeout is a timeout error
var ErrTimeout = errors.New("timeout")

// Storage is a chunk storage
type Storage struct {
	ChunkSize int64
	MaxChunks int
	chunks    map[string][]byte
	stack     *Stack
	lock      sync.RWMutex
}

// Item represents a chunk in RAM
type Item struct {
	id    string
	bytes []byte
}

// NewStorage creates a new storage
func NewStorage(chunkSize int64, maxChunks int) *Storage {
	storage := Storage{
		ChunkSize: chunkSize,
		MaxChunks: maxChunks,
		chunks:    make(map[string][]byte),
		stack:     NewStack(maxChunks),
	}

	return &storage
}

// Clear removes all old chunks on disk (will be called on each program start)
func (s *Storage) Clear() error {
	return nil
}

// Load a chunk from ram or creates it
func (s *Storage) Load(id string) []byte {
	s.lock.RLock()
	if chunk, exists := s.chunks[id]; exists {
		s.stack.Touch(id)
		s.lock.RUnlock()
		return chunk
	}
	s.lock.RUnlock()
	return nil
}

// Store stores a chunk in the RAM and adds it to the disk storage queue
func (s *Storage) Store(id string, bytes []byte) error {
	s.lock.RLock()

	if _, exists := s.chunks[id]; exists {
		s.stack.Touch(id)
		s.lock.RUnlock()
		return nil
	}

	s.lock.RUnlock()
	s.lock.Lock()

	deleteID := s.stack.Pop()
	if "" != deleteID {
		delete(s.chunks, deleteID)

		Log.Debugf("Deleted chunk %v", deleteID)
	}

	s.chunks[id] = bytes
	s.stack.Push(id)
	s.lock.Unlock()

	return nil
}
