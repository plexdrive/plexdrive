package chunk

import (
	"errors"
	"fmt"
	"os"
	"sync"
	// . "github.com/claudetech/loggo/default"
)

// ErrTimeout is a timeout error
var ErrTimeout = errors.New("timeout")

// Storage is a chunk storage
type Storage struct {
	ChunkPath string
	ChunkSize int64
	MaxChunks int
	// queue      chan *Item
	chunks map[string][]byte
	lock   sync.RWMutex
	// toc        map[string]error
	// tocLock    sync.RWMutex
	// stack *Stack
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
		// queue:     make(chan *Item, 100),
		chunks: make(map[string][]byte),
		// toc:    make(map[string]error),
		// stack: NewStack(),
	}

	// go storage.thread()

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

// func (s *Storage) thread() {
// 	for {
// 		item := <-s.queue
// 		if err := s.storeToDisk(item.id, item.bytes); nil != err {
// 			Log.Warningf("%v", err)
// 		}
// 	}
// }

// func (s *Storage) deleteFromToc(id string) {
// 	s.tocLock.Lock()
// 	delete(s.toc, id)
// 	s.tocLock.Unlock()
// }

// func (s *Storage) loadFromRAM(id string, offset, size int64) ([]byte, bool) {
// 	s.chunksLock.RLock()
// 	bytes, exists := s.chunks[id]
// 	s.chunksLock.RUnlock()
// 	if !exists {
// 		return nil, false
// 	}

// 	sOffset := int64(math.Min(float64(len(bytes)), float64(offset)))
// 	eOffset := int64(math.Min(float64(len(bytes)), float64(offset+size)))
// 	return bytes[sOffset:eOffset], true
// }

// func (s *Storage) loadFromDisk(id string, offset, size int64) ([]byte, bool) {
// 	filename := filepath.Join(s.ChunkPath, id)

// 	f, err := os.Open(filename)
// 	if nil != err {
// 		Log.Tracef("%v", err)
// 		return nil, false
// 	}
// 	defer f.Close()

// 	buf := make([]byte, size)
// 	n, err := f.ReadAt(buf, offset)
// 	if n > 0 && (nil == err || io.EOF == err || io.ErrUnexpectedEOF == err) {
// 		s.stack.Touch(id)

// 		eOffset := int64(math.Min(float64(size), float64(n)))
// 		return buf[:eOffset], true
// 	}

// 	Log.Tracef("%v", err)
// 	return nil, false
// }

// func (s *Storage) storeToDisk(id string, bytes []byte) error {
// 	filename := filepath.Join(s.ChunkPath, id)

// 	if s.stack.Len() >= s.MaxChunks {
// 		deleteID := s.stack.Pop()

// 		if "" != deleteID {
// 			filename := filepath.Join(s.ChunkPath, deleteID)

// 			Log.Debugf("Deleting chunk %v", filename)
// 			if err := os.Remove(filename); nil != err {
// 				Log.Debugf("%v", err)
// 				Log.Warningf("Could not delete chunk %v", filename)
// 			}

// 			s.tocLock.Lock()
// 			delete(s.toc, deleteID)
// 			s.tocLock.Unlock()
// 		}
// 	}

// 	if _, err := os.Stat(s.ChunkPath); os.IsNotExist(err) {
// 		if err := os.MkdirAll(s.ChunkPath, 0777); nil != err {
// 			Log.Debugf("%v", err)
// 			return fmt.Errorf("Could not create chunk temp path %v", s.ChunkPath)
// 		}
// 	}

// 	if _, err := os.Stat(filename); os.IsNotExist(err) {
// 		if err := ioutil.WriteFile(filename, bytes, 0777); nil != err {
// 			Log.Debugf("%v", err)
// 			return fmt.Errorf("Could not write chunk temp file %v", filename)
// 		}

// 		s.stack.Push(id)
// 	}

// 	s.chunksLock.Lock()
// 	delete(s.chunks, id)
// 	s.chunksLock.Unlock()

// 	return nil
// }
