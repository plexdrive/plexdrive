package chunk

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

// ErrTimeout is a timeout error
var ErrTimeout = errors.New("timeout")

// Storage is a chunk storage
type Storage struct {
	ChunkPath string
	ChunkSize int64
	MaxChunks int
	chunks    map[string][]byte
	lock      sync.Mutex
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

// Load a chunk from ram or creates it
func (s *Storage) Load(id string) []byte {
	s.lock.Lock()
	if chunk, exists := s.chunks[id]; exists {
		s.lock.Unlock()
		return chunk
	}
	s.lock.Unlock()
	return nil
}

// Store stores a chunk in the RAM and adds it to the disk storage queue
func (s *Storage) Store(id string, bytes []byte) error {
	s.lock.Lock()

	// // delete chunk
	// for s.stackSize > s.MaxChunks {
	// 	Log.Debugf("%v / %v", s.stackSize, s.MaxChunks)

	// 	deleteID := s.stack[0]
	// 	s.stack = s.stack[1:]
	// 	s.stackSize--

	// 	Log.Debugf("Deleting chunk %v", deleteID)
	// 	delete(s.chunks, deleteID)
	// }

	s.chunks[id] = bytes
	s.lock.Unlock()

	return nil
}
