package chunk

import (
	"container/list"
	"fmt"
	"hash/crc32"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	. "github.com/claudetech/loggo/default"
)

const (
	headerSize     = int(unsafe.Sizeof(*new(chunkHeader)))
	tocSize        = int64(unsafe.Sizeof(*new(journalHeader)))
	journalMagic   = uint16('P'<<8 | 'D'&0xFF)
	journalVersion = uint8(2)
)

var (
	blankRequestID RequestID
	pageSize       = int64(os.Getpagesize())
	crc32Table     = crc32.MakeTable(crc32.Castagnoli)
)

// Storage is a chunk storage
type Storage struct {
	ChunkFile  *os.File
	ChunkSize  int64
	HeaderSize int64
	MaxChunks  int
	chunks     map[RequestID]int
	stack      *Stack
	lock       sync.RWMutex
	buffers    []*Chunk
	loadChunks int
	lastIndex  int
	signals    chan os.Signal
	journal    []byte
}

type journalHeader struct {
	magic      uint16
	version    uint8
	headerSize uint8
	maxChunks  uint32
	chunkSize  uint32
	checksum   uint32
}

// NewStorage creates a new storage
func NewStorage(chunkSize int64, maxChunks int, chunkFilePath string) (*Storage, error) {
	s := Storage{
		ChunkSize: chunkSize,
		MaxChunks: maxChunks,
		chunks:    make(map[RequestID]int, maxChunks),
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

	h := (*journalHeader)(unsafe.Pointer(&header[0]))
	oldJournalOffset := s.ChunkSize * int64(h.maxChunks)
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
	h := (*journalHeader)(unsafe.Pointer(&journal[0]))
	// check magic bytes / endianess mismatch ('PD' vs 'DP')
	if h.magic != journalMagic {
		return false
	}
	if 0 == h.version || 0 == h.checksum {
		// assume unitialized memory
		return false
	}
	if h.checksum != crc32.Checksum(journal[:12], crc32Table) {
		return false
	}
	if h.version != journalVersion {
		return false
	}
	if h.headerSize != uint8(headerSize) {
		return false
	}
	if !skipMaxChunks && h.maxChunks != uint32(s.MaxChunks) {
		return false
	}
	if h.chunkSize != uint32(s.ChunkSize) {
		return false
	}
	return true
}

// initJournal initializes the journal
func (s *Storage) initJournal(journal []byte) {
	h := (*journalHeader)(unsafe.Pointer(&journal[0]))
	h.magic = journalMagic
	h.version = journalVersion
	h.headerSize = uint8(headerSize)
	h.maxChunks = uint32(s.MaxChunks)
	h.chunkSize = uint32(s.ChunkSize)
	h.checksum = crc32.Checksum(journal[:12], crc32Table)
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

	id := chunk.id

	if blankRequestID == id || index >= s.loadChunks {
		chunk.item = empty.PushBack(index)
		Log.Tracef("Allocate chunk %v/%v", index+1, s.MaxChunks)
		return false, nil
	}

	chunk.item = restored.PushBack(index)
	Log.Tracef("Load chunk %v/%v (restored: %v)", index+1, s.MaxChunks, id)
	s.chunks[id] = index

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
	header := (*chunkHeader)(unsafe.Pointer(&s.journal[headerOffset]))
	chunk := Chunk{
		chunkHeader: header,
		bytes:       bytes,
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
	s.lock.RLock()
	chunk := s.fetch(id)
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
	if chunk.valid(id) {
		Log.Debugf("Load chunk %v (verified)", id)
		return chunk.bytes
	}
	Log.Warningf("Load chunk %v (bad checksum: %08x <> %08x)", id, chunk.checksum, chunk.calculateChecksum())
	s.stack.Purge(chunk.item)
	return nil
}

// Store stores a chunk in the RAM and adds it to the disk storage queue
func (s *Storage) Store(id RequestID, bytes []byte) (err error) {
	s.lock.RLock()

	// Avoid storing same chunk multiple times
	chunk := s.fetch(id)
	if nil != chunk && chunk.clean {
		Log.Tracef("Create chunk %v (exists: clean)", id)
		s.lock.RUnlock()
		return nil
	}

	s.lock.RUnlock()
	s.lock.Lock()
	defer s.lock.Unlock()

	if nil != chunk {
		if chunk.valid(id) {
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
		deleteID := chunk.id
		if blankRequestID != deleteID {
			delete(s.chunks, deleteID)
			Log.Debugf("Create chunk %v (reused)", id)
		} else {
			Log.Debugf("Create chunk %v (stored)", id)
		}
		s.chunks[id] = index
		chunk.item = s.stack.Push(index)
	}

	chunk.update(id, bytes)

	return nil
}

// fetch chunk and index by id
func (s *Storage) fetch(id RequestID) *Chunk {
	index, exists := s.chunks[id]
	if !exists {
		return nil
	}
	chunk := s.buffers[index]
	s.stack.Touch(chunk.item)
	return chunk
}
