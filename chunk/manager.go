package chunk

import (
	"fmt"

	. "github.com/claudetech/loggo/default"

	"time"

	"github.com/dweidenfeld/plexdrive/drive"
)

// Manager manages chunks on disk
type Manager struct {
	ChunkPath      string
	ChunkSize      int64
	LoadAhead      int
	Timeout        time.Duration
	TimeoutRetries int
	downloader     *Downloader
	storage        *Storage
	preloadQueue   chan *Request
}

// Request represents a chunk request
type Request struct {
	id             string
	object         *drive.APIObject
	offsetStart    int64
	offsetEnd      int64
	chunkOffset    int64
	chunkOffsetEnd int64
}

type ResponseFunc func(error, []byte)

// NewManager creates a new chunk manager
func NewManager(
	chunkPath string,
	chunkSize int64,
	loadAhead,
	threads int,
	client *drive.Client,
	maxChunks int,
	timeout time.Duration,
	timeoutRetries int) (*Manager, error) {

	if "" == chunkPath {
		return nil, fmt.Errorf("Path to chunk file must not be empty")
	}
	if chunkSize < 4096 {
		return nil, fmt.Errorf("Chunk size must not be < 4096")
	}
	if chunkSize%1024 != 0 {
		return nil, fmt.Errorf("Chunk size must be divideable by 1024")
	}
	if maxChunks < 2 || maxChunks < loadAhead {
		return nil, fmt.Errorf("max-chunk must be greater than 2 and bigger than the load ahead value")
	}

	downloader, err := NewDownloader(threads, client)
	if nil != err {
		return nil, err
	}

	manager := Manager{
		ChunkPath:      chunkPath,
		ChunkSize:      chunkSize,
		LoadAhead:      loadAhead,
		Timeout:        timeout,
		TimeoutRetries: timeoutRetries,
		downloader:     downloader,
		storage:        NewStorage(chunkPath, chunkSize, maxChunks),
	}

	if err := manager.storage.Clear(); nil != err {
		return nil, err
	}

	go manager.thread()

	return &manager, nil
}

// GetChunk loads one chunk and starts the preload for the next chunks
func (m *Manager) GetChunk(object *drive.APIObject, offset, size int64, callback ResponseFunc) {
	chunkOffset := offset % m.ChunkSize
	offsetStart := offset - chunkOffset
	offsetEnd := offsetStart + m.ChunkSize
	id := fmt.Sprintf("%v:%v", object.ObjectID, offsetStart)

	request := &Request{
		id:             id,
		object:         object,
		offsetStart:    offsetStart,
		offsetEnd:      offsetEnd,
		chunkOffset:    chunkOffset,
		chunkOffsetEnd: chunkOffset + size,
	}

	m.checkChunk(request, callback)

	// for i := m.ChunkSize; i < (m.ChunkSize * int64(m.LoadAhead+1)); i += m.ChunkSize {
	// 	aheadOffsetStart := offsetStart + i
	// 	aheadOffsetEnd := aheadOffsetStart + m.ChunkSize
	// 	if uint64(aheadOffsetStart) < object.Size && uint64(aheadOffsetEnd) < object.Size {
	// 		id := fmt.Sprintf("%v:%v", object.ObjectID, aheadOffsetStart)
	// 		m.preloadQueue <- &Request{
	// 			id:          id,
	// 			object:      object,
	// 			offsetStart: aheadOffsetStart,
	// 			offsetEnd:   aheadOffsetEnd,
	// 		}
	// 	}
	// }
}

func (m *Manager) thread() {
	for {
		req := <-m.preloadQueue
		m.checkChunk(req, nil)
	}
}

func (m *Manager) checkChunk(req *Request, callback ResponseFunc) {
	if chunk := m.storage.Load(req.id); nil != chunk {
		if nil != callback {
			callback(nil, chunk[req.chunkOffset:req.chunkOffsetEnd])
		}
		return
	}

	m.downloader.Download(req, func(err error, bytes []byte) {
		if nil != callback {
			callback(err, bytes[req.chunkOffset:req.chunkOffsetEnd])
		}

		if nil != err {
			if err := m.storage.Store(req.id, bytes); nil != err {
				Log.Warningf("Could not store chunk %v", req.id)
			}
		}
	})
}
