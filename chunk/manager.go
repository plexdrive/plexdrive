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
	queue          chan *QueueEntry
}

type QueueEntry struct {
	request  *Request
	response chan Response
}

// Request represents a chunk request
type Request struct {
	id             string
	object         *drive.APIObject
	offsetStart    int64
	offsetEnd      int64
	chunkOffset    int64
	chunkOffsetEnd int64
	preload        bool
}

// Response represetns a chunk response
type Response struct {
	Error error
	Bytes []byte
}

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
		queue:          make(chan *QueueEntry, 100),
	}

	if err := manager.storage.Clear(); nil != err {
		return nil, err
	}

	for i := 0; i < threads; i++ {
		go manager.thread(i)
	}

	return &manager, nil
}

// GetChunk loads one chunk and starts the preload for the next chunks
func (m *Manager) GetChunk(object *drive.APIObject, offset, size int64, response chan Response) {
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
		preload:        false,
	}

	m.queue <- &QueueEntry{
		request:  request,
		response: response,
	}

	for i := m.ChunkSize; i < (m.ChunkSize * int64(m.LoadAhead+1)); i += m.ChunkSize {
		aheadOffsetStart := offsetStart + i
		aheadOffsetEnd := aheadOffsetStart + m.ChunkSize
		if uint64(aheadOffsetStart) < object.Size && uint64(aheadOffsetEnd) < object.Size {
			id := fmt.Sprintf("%v:%v", object.ObjectID, aheadOffsetStart)
			request := &Request{
				id:          id,
				object:      object,
				offsetStart: aheadOffsetStart,
				offsetEnd:   aheadOffsetEnd,
				preload:     true,
			}
			m.queue <- &QueueEntry{
				request: request,
			}
		}
	}
}

func (m *Manager) thread(threadID int) {
	for {
		queueEntry := <-m.queue
		m.checkChunk(queueEntry.request, queueEntry.response)
	}
}

func (m *Manager) checkChunk(req *Request, response chan Response) {
	if chunk := m.storage.Load(req.id); nil != chunk {
		if nil != response {
			response <- Response{
				Bytes: chunk[req.chunkOffset:req.chunkOffsetEnd],
			}
			close(response)
		}
		return
	}

	m.downloader.Download(req, func(err error, bytes []byte) {
		if nil != err {
			if nil != response {
				response <- Response{
					Error: err,
				}
				close(response)
			}
			return
		}

		if nil != response {
			response <- Response{
				Bytes: bytes[req.chunkOffset:req.chunkOffsetEnd],
			}
			close(response)
		}

		if err := m.storage.Store(req.id, bytes); nil != err {
			Log.Warningf("Coult not store chunk %v", req.id)
		}
	})
}
