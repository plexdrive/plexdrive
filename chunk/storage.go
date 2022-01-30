package chunk

import (
	"container/list"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"hash/crc64"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	. "github.com/claudetech/loggo/default"
)

// ErrTimeout is a timeout error
var ErrTimeout = errors.New("timeout")

var (
	pageSize   = int64(os.Getpagesize())
	headerSize = 32
	tocSize    = int64(16)
	journalVer = uint8(2)
	crc32Table = crc32.MakeTable(crc32.Castagnoli)
	crc64Table = crc64.MakeTable(crc64.ECMA)
	blankIDKey = uint64(13156847312662197100)
)

// Storage is a chunk storage
type Storage struct {
	ChunkFile  *os.File
	ChunkSize  int64
	HeaderSize int64
	MaxChunks  int
	chunks     map[uint64]int
	stack      *Stack
	lock       sync.RWMutex
	buffers    []*Chunk
	loadChunks int
	lastIndex  int
	signals    chan os.Signal
	journal    []byte
}

// NewStorage creates a new storage
func NewStorage(chunkSize int64, maxChunks int, chunkFilePath string) (*Storage, error) {
	s := Storage{
		ChunkSize: chunkSize,
		MaxChunks: maxChunks,
		chunks:    make(map[uint64]int, maxChunks),
		stack:     NewStack(maxChunks),
		buffers:   make([]*Chunk, maxChunks, maxChunks),
		signals:   make(chan os.Signal, 1),
	}

	journalSize := tocSize + int64(headerSize*maxChunks)
	if rem := journalSize % pageSize; 0 != rem {
		journalSize += pageSize - rem
	}
	journalOffset := chunkSize * int64(maxChunks)

	// Non-empty string in chunkFilePath enables MMAP disk storage for chunks
	if chunkFilePath != "" {
		chunkFile, err := os.OpenFile(chunkFilePath, os.O_RDWR|os.O_CREATE, 0600)
		if nil != err {
			Log.Debugf("%v", err)
			return nil, fmt.Errorf("Could not open chunk cache file")
		}
		stat, err := chunkFile.Stat()
		if nil != err {
			Log.Debugf("%v", err)
			return nil, fmt.Errorf("Could not stat chunk cache file")
		}
		s.ChunkFile = chunkFile
		currentSize := stat.Size()
		wantedSize := journalOffset + journalSize
		if currentSize != wantedSize {
			if currentSize > tocSize {
				err = s.relocateJournal(currentSize, wantedSize, journalSize, journalOffset)
				if nil != err {
					Log.Errorf("%v", err)
				} else {
					Log.Infof("Relocated chunk cache journal")
				}
			}
			err = chunkFile.Truncate(wantedSize)
			if nil != err {
				Log.Debugf("%v", err)
				return nil, fmt.Errorf("Could not resize chunk cache file")
			}
		}
		Log.Infof("Created chunk cache file %v", chunkFile.Name())
		s.loadChunks = int(min(currentSize/chunkSize, int64(maxChunks)))
	}

	// Alocate journal
	if journal, err := s.mmap(journalOffset, journalSize); nil != err {
		return nil, fmt.Errorf("Could not allocate journal: %v", err)
	} else {
		unix.Madvise(journal, syscall.MADV_RANDOM)
		tocOffset := journalSize - tocSize
		header := journal[tocOffset:]
		if valid := s.checkJournal(header, false); !valid {
			s.initJournal(header)
		}
		s.journal = journal[:tocOffset]
	}

	// Setup sighandler
	signal.Notify(s.signals, syscall.SIGINT, syscall.SIGTERM)

	// Initialize chunks
	if err := s.mmapChunks(); nil != err {
		return nil, err
	}

	return &s, nil
}

// relocateJournal moves existing journal prior to resize
func (s *Storage) relocateJournal(currentSize, wantedSize, journalSize, journalOffset int64) error {
	header := make([]byte, tocSize, tocSize)
	if _, err := s.ChunkFile.ReadAt(header, currentSize-tocSize); nil != err {
		return fmt.Errorf("Failed to read journal header: %v", err)
	}

	if valid := s.checkJournal(header, true); !valid {
		return fmt.Errorf("Failed to validate journal header")
	}

	oldMaxChunks := int64(binary.LittleEndian.Uint32(header[4:]))
	oldJournalOffset := s.ChunkSize * int64(oldMaxChunks)
	oldJournalSize := min(journalSize, currentSize-oldJournalOffset) - tocSize
	journal := make([]byte, journalSize, journalSize)

	if _, err := s.ChunkFile.ReadAt(journal[:oldJournalSize], oldJournalOffset); nil != err {
		return fmt.Errorf("Failed to read journal: %v", err)
	}

	s.initJournal(header)

	if err := s.ChunkFile.Truncate(currentSize - oldJournalSize - tocSize); nil != err {
		return fmt.Errorf("Could not truncate chunk cache journal: %v", err)
	}

	if err := s.ChunkFile.Truncate(wantedSize); nil != err {
		return fmt.Errorf("Could not resize chunk cache file: %v", err)
	}

	if _, err := s.ChunkFile.WriteAt(journal, journalOffset); nil != err {
		return fmt.Errorf("Failed to write journal: %v", err)
	}
	if _, err := s.ChunkFile.WriteAt(header, wantedSize-tocSize); nil != err {
		return fmt.Errorf("Failed to write journal header: %v", err)
	}
	return nil
}

// checkJournal verifies the journal header
func (s *Storage) checkJournal(journal []byte, skipMaxChunks bool) bool {
	// check magic bytes
	if journal[0] != 'P' || journal[1] != 'D' {
		return false
	}
	version := uint8(journal[2])
	checksum := binary.LittleEndian.Uint32(journal[12:])
	if 0 == version || 0 == checksum {
		// assume unitialized memory
		return false
	}
	if checksum != crc32.Checksum(journal[:12], crc32Table) {
		return false
	}
	if version != journalVer {
		return false
	}
	header := int(journal[3])
	if header != headerSize {
		return false
	}
	maxChunks := int(binary.LittleEndian.Uint32(journal[4:]))
	if !skipMaxChunks && maxChunks != s.MaxChunks {
		return false
	}
	chunkSize := int64(binary.LittleEndian.Uint32(journal[8:]))
	if chunkSize != s.ChunkSize {
		return false
	}
	return true
}

// initJournal initializes the journal
func (s *Storage) initJournal(journal []byte) {
	journal[0] = 'P'
	journal[1] = 'D'
	journal[2] = uint8(journalVer)
	journal[3] = uint8(headerSize)
	binary.LittleEndian.PutUint32(journal[4:], uint32(s.MaxChunks))
	binary.LittleEndian.PutUint32(journal[8:], uint32(s.ChunkSize))
	checksum := crc32.Checksum(journal[:12], crc32Table)
	binary.LittleEndian.PutUint32(journal[12:], checksum)
}

// mmapChunks mmaps buffers and loads chunk metadata
func (s *Storage) mmapChunks() error {
	start := time.Now()
	empty := list.New()
	restored := list.New()
	loadedChunks := 0
	for i := 0; i < s.MaxChunks; i++ {
		select {
		case sig := <-s.signals:
			Log.Warningf("Received signal %v, aborting chunk loader", sig)
			return fmt.Errorf("Aborted by signal")
		default:
			if loaded, err := s.initChunk(i, empty, restored); nil != err {
				Log.Errorf("Failed to allocate chunk %v: %v", i, err)
				return fmt.Errorf("Failed to initialize chunks")
			} else if loaded {
				loadedChunks++
			}
		}
	}
	s.stack.Prepend(restored)
	s.stack.Prepend(empty)
	elapsed := time.Since(start)
	if nil != s.ChunkFile {
		Log.Infof("Loaded %v/%v cache chunks in %v", loadedChunks, s.MaxChunks, elapsed)
	} else {
		Log.Infof("Allocated %v cache chunks in %v", s.MaxChunks, elapsed)
	}
	return nil
}

// initChunk tries to restore a chunk from disk
func (s *Storage) initChunk(index int, empty *list.List, restored *list.List) (bool, error) {
	chunk, err := s.allocateChunk(index)
	if err != nil {
		Log.Debugf("%v", err)
		return false, err
	}

	s.buffers[index] = chunk

	id := chunk.ID()
	key := idToKey(id)

	if blankIDKey == key || index >= s.loadChunks {
		chunk.item = empty.PushBack(index)
		Log.Tracef("Allocate chunk %v/%v", index+1, s.MaxChunks)
		return false, nil
	}

	chunk.item = restored.PushBack(index)
	Log.Tracef("Load chunk %v/%v (restored: %v)", index+1, s.MaxChunks, id)
	s.chunks[key] = index

	return true, nil
}

// allocateChunk creates a new mmap-backed chunk
func (s *Storage) allocateChunk(index int) (*Chunk, error) {
	Log.Tracef("Mmap chunk %v/%v", index+1, s.MaxChunks)
	offset := int64(index) * s.ChunkSize
	bytes, err := s.mmap(offset, s.ChunkSize)
	if nil != err {
		return nil, err
	}
	unix.Madvise(bytes, syscall.MADV_SEQUENTIAL)
	headerOffset := index * headerSize
	header := s.journal[headerOffset : headerOffset+headerSize : headerOffset+headerSize]
	chunk := Chunk{
		header: header,
		bytes:  bytes,
	}
	return &chunk, nil
}

func (s *Storage) mmap(offset, size int64) ([]byte, error) {
	if s.ChunkFile != nil {
		return unix.Mmap(int(s.ChunkFile.Fd()), offset, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	} else {
		return unix.Mmap(-1, 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	}
}

// Clear removes all old chunks on disk (will be called on each program start)
func (s *Storage) Clear() error {
	return nil
}

// Load a chunk from ram or creates it
func (s *Storage) Load(id RequestID) []byte {
	key := idToKey(id)
	s.lock.RLock()
	chunk := s.fetch(key)
	if nil == chunk {
		Log.Tracef("Load chunk %v (missing)", id)
		s.lock.RUnlock()
		return nil
	}
	if chunk.clean {
		Log.Tracef("Load chunk %v (clean)", id)
		defer s.lock.RUnlock()
		return chunk.bytes
	}
	s.lock.RUnlock()
	// Switch to write lock to avoid races on crc verification
	s.lock.Lock()
	defer s.lock.Unlock()
	if chunk.Valid(key) {
		Log.Debugf("Load chunk %v (verified)", id)
		return chunk.bytes
	}
	Log.Warningf("Load chunk %v (bad checksum: %08x <> %08x)", id, chunk.Checksum(), chunk.calculateChecksum())
	s.stack.Purge(chunk.item)
	return nil
}

// Store stores a chunk in the RAM and adds it to the disk storage queue
func (s *Storage) Store(id RequestID, bytes []byte) (err error) {
	key := idToKey(id)
	s.lock.RLock()

	// Avoid storing same chunk multiple times
	chunk := s.fetch(key)
	if nil != chunk && chunk.clean {
		Log.Tracef("Create chunk %v (exists: clean)", id)
		s.lock.RUnlock()
		return nil
	}

	s.lock.RUnlock()
	s.lock.Lock()
	defer s.lock.Unlock()

	if nil != chunk {
		if chunk.Valid(key) {
			Log.Debugf("Create chunk %v (exists: valid)", id)
			return nil
		}
		Log.Warningf("Create chunk %v(exists: overwrite)", id)
	} else {
		index := s.stack.Pop()
		if -1 == index {
			Log.Debugf("Create chunk %v (failed)", id)
			return fmt.Errorf("No buffers available")
		}
		chunk = s.buffers[index]
		deleteKey := idToKey(chunk.ID())
		if blankIDKey != deleteKey {
			delete(s.chunks, deleteKey)
			Log.Debugf("Create chunk %v (reused)", id)
		} else {
			Log.Debugf("Create chunk %v (stored)", id)
		}
		s.chunks[key] = index
		chunk.item = s.stack.Push(index)
	}

	chunk.Update(id, bytes)

	return nil
}

// fetch chunk and index by id
func (s *Storage) fetch(key uint64) *Chunk {
	index, exists := s.chunks[key]
	if !exists {
		return nil
	}
	chunk := s.buffers[index]
	s.stack.Touch(chunk.item)
	return chunk
}

// idToKey converts binary id to internal uint64 representation
func idToKey(id RequestID) uint64 {
	return crc64.Checksum(id[:], crc64Table)
}
