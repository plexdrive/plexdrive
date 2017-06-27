package chunk

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sync"

	"time"

	"sort"

	. "github.com/claudetech/loggo/default"
)

type Storage struct {
	ChunkPath  string
	ChunkSize  int64
	MaxChunks  int
	queue      chan *Item
	chunks     map[string][]byte
	chunksLock sync.Mutex
	toc        map[string]time.Time
	tocLock    sync.Mutex
}

type Item struct {
	id    string
	bytes []byte
}

type SortChunk struct {
	id     string
	access time.Time
}

func NewStorage(chunkPath string, chunkSize int64, maxChunks int) *Storage {
	storage := Storage{
		ChunkPath: chunkPath,
		ChunkSize: chunkSize,
		MaxChunks: maxChunks,
		queue:     make(chan *Item, 100),
		chunks:    make(map[string][]byte),
		toc:       make(map[string]time.Time),
	}

	go storage.thread()
	go storage.cleanThread()

	return &storage
}

func (s *Storage) Clear() error {
	if err := os.RemoveAll(s.ChunkPath); nil != err {
		return fmt.Errorf("Could not clear old chunks from disk")
	}
	return nil
}

func (s *Storage) ExistsOrCreate(id string) bool {
	s.tocLock.Lock()
	if _, exists := s.toc[id]; exists {
		s.tocLock.Unlock()
		return true
	}
	s.toc[id] = time.Now()
	s.tocLock.Unlock()
	return false
}

func (s *Storage) Store(id string, bytes []byte) error {
	s.chunksLock.Lock()
	s.chunks[id] = bytes
	s.chunksLock.Unlock()

	s.queue <- &Item{
		id:    id,
		bytes: bytes,
	}

	return nil
}

func (s *Storage) Get(id string, offset, size int64) ([]byte, error) {
	res := make(chan []byte)

	go func() {
		for {
			s.tocLock.Lock()
			_, exists := s.toc[id]
			s.tocLock.Unlock()
			if exists {
				bytes, exists := s.loadFromRAM(id, offset, size)
				if exists {
					res <- bytes
					return
				}

				bytes, exists = s.loadFromDisk(id, offset, size)
				if exists {
					res <- bytes
					return
				}
			}

			time.Sleep(10 * time.Millisecond)
		}
	}()

	return <-res, nil
}

func (s *Storage) thread() {
	for {
		item := <-s.queue
		if err := s.storeToDisk(item.id, item.bytes); nil != err {
			Log.Warningf("%v", err)
		}
	}
}

func (s *Storage) cleanThread() {
	for _ = range time.Tick(1 * time.Second) {
		s.deleteChunks()
	}
}

func (s *Storage) loadFromRAM(id string, offset, size int64) ([]byte, bool) {
	bytes, exists := s.chunks[id]
	if !exists {
		return nil, false
	}

	sOffset := int64(math.Min(float64(len(bytes)), float64(offset)))
	eOffset := int64(math.Min(float64(len(bytes)), float64(offset+size)))
	return bytes[sOffset:eOffset], true
}

func (s *Storage) loadFromDisk(id string, offset, size int64) ([]byte, bool) {
	filename := filepath.Join(s.ChunkPath, id)

	f, err := os.Open(filename)
	if nil != err {
		Log.Tracef("%v", err)
		return nil, false
	}
	defer f.Close()

	buf := make([]byte, size)
	n, err := f.ReadAt(buf, offset)
	if n > 0 && (nil == err || io.EOF == err || io.ErrUnexpectedEOF == err) {
		eOffset := int64(math.Min(float64(size), float64(len(buf))))
		return buf[:eOffset], true
	}

	Log.Tracef("%v", err)
	return nil, false
}

func (s *Storage) storeToDisk(id string, bytes []byte) error {
	filename := filepath.Join(s.ChunkPath, id)

	if _, err := os.Stat(s.ChunkPath); os.IsNotExist(err) {
		if err := os.MkdirAll(s.ChunkPath, 0777); nil != err {
			Log.Debugf("%v", err)
			return fmt.Errorf("Could not create chunk temp path %v", s.ChunkPath)
		}
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if err := ioutil.WriteFile(filename, bytes, 0777); nil != err {
			Log.Debugf("%v", err)
			return fmt.Errorf("Could not write chunk temp file %v", filename)
		}
	}

	s.chunksLock.Lock()
	delete(s.chunks, id)
	s.chunksLock.Unlock()

	return nil
}

func (s *Storage) deleteChunks() {
	var chunkList []*SortChunk
	filepath.Walk(s.ChunkPath, func(path string, f os.FileInfo, err error) error {
		if nil != err {
			Log.Tracef("%v", err)
			return filepath.SkipDir
		}
		if nil == f {
			return filepath.SkipDir
		}

		if !f.IsDir() {
			chunkList = append(chunkList, &SortChunk{
				id:     f.Name(),
				access: f.ModTime(),
			})
		}
		return nil
	})

	length := len(chunkList)

	if length > s.MaxChunks {
		sort.Slice(chunkList, func(a, b int) bool {
			return chunkList[a].access.Before(chunkList[b].access)
		})

		for i := 0; i < s.MaxChunks-length; i++ {
			chunk := chunkList[i]

			filename := filepath.Join(s.ChunkPath, chunk.id)
			if "" != filename {
				Log.Debugf("Deleting chunk %v", filename)

				s.tocLock.Lock()
				delete(s.toc, chunk.id)
				s.tocLock.Unlock()

				if err := os.Remove(filename); nil != err {
					Log.Debugf("%v", err)
					Log.Warningf("Could not delete chunk %v", filename)
				}
			}
		}
	}
}
