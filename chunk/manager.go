package chunk

import (
	"fmt"

	"net/http"

	"time"

	"github.com/dweidenfeld/plexdrive/alog"
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
	queue          chan *Request
	preloadQueue   chan *Request
	storage        *Storage
}

// Request represents a chunk request
type Request struct {
	id          string
	object      *drive.APIObject
	preload     bool
	offsetStart int64
	offsetEnd   int64
}

// NewManager creates a new chunk manager
func NewManager(
	chunkPath string,
	chunkSize int64,
	loadAhead,
	threads int,
	client *http.Client,
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
		queue:          make(chan *Request, 100),
		preloadQueue:   make(chan *Request, 100),
		storage:        NewStorage(chunkPath, chunkSize, maxChunks),
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
func (m *Manager) GetChunk(object *drive.APIObject, offset, size int64) ([]byte, error) {
	chunkOffset := offset % m.ChunkSize
	offsetStart := offset - chunkOffset
	offsetEnd := offsetStart + m.ChunkSize
	id := fmt.Sprintf("%v:%v", object.ObjectID, offsetStart)

	m.queue <- &Request{
		id:          id,
		object:      object,
		offsetStart: offsetStart,
		offsetEnd:   offsetEnd,
		preload:     false,
	}

	for i := m.ChunkSize; i < (m.ChunkSize * int64(m.LoadAhead+1)); i += m.ChunkSize {
		aheadOffsetStart := offsetStart + i
		aheadOffsetEnd := aheadOffsetStart + m.ChunkSize
		if uint64(aheadOffsetStart) < object.Size && uint64(aheadOffsetEnd) < object.Size {
			id := fmt.Sprintf("%v:%v", object.ObjectID, aheadOffsetStart)
			m.preloadQueue <- &Request{
				id:          id,
				object:      object,
				offsetStart: aheadOffsetStart,
				offsetEnd:   aheadOffsetEnd,
				preload:     true,
			}
		}
	}

	bytes, err := m.storage.Get(object, id, chunkOffset, size, m.Timeout)
	retryCount := 0
	for err == ErrTimeout && retryCount < m.TimeoutRetries {
		alog.Warn(map[string]interface{}{
			"ObjectID":     object.ObjectID,
			"ObjectName":   object.Name,
			"ID":           id,
			"Retry":        (retryCount + 1),
			"RetryMaximum": m.TimeoutRetries,
		}, "Timeout while requesting chunk")
		bytes, err = m.storage.Get(object, id, chunkOffset, size, m.Timeout)
		retryCount++
	}
	return bytes, err
}

func (m *Manager) thread(id int) {
	for {
		select {
		case req := <-m.queue:
			m.checkChunk(req, id)
			break
		case req := <-m.preloadQueue:
			m.checkChunk(req, id)
			break
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (m *Manager) checkChunk(req *Request, threadID int) {
	if m.storage.ExistsOrCreate(req.id) {
		return
	}

	alog.Debug(map[string]interface{}{
		"ObjectID":   req.object.ObjectID,
		"ObjectName": req.object.Name,
		"ID":         req.id,
		"Preload":    req.preload,
		"ThreadID":   threadID,
	}, "Got chunk checking request")

	before := time.Now()
	bytes, err := m.downloader.Download(req)
	if nil != err {
		alog.Warn(map[string]interface{}{
			"ObjectID":   req.object.ObjectID,
			"ObjectName": req.object.Name,
			"ID":         req.id,
			"Preload":    req.preload,
			"Error":      err,
		}, "Could not download chunk")
		m.storage.Error(req.id, err)
		return
	}

	alog.Debug(map[string]interface{}{
		"ObjectID":   req.object.ObjectID,
		"ObjectName": req.object.Name,
		"ID":         req.id,
		"Preload":    req.preload,
		"Took":       time.Now().Sub(before),
	}, "Download Time")

	if err := m.storage.Store(req.object, req.id, bytes); nil != err {
		alog.Warn(map[string]interface{}{
			"ObjectID":   req.object.ObjectID,
			"ObjectName": req.object.Name,
			"ID":         req.id,
			"Preload":    req.preload,
			"Error":      err,
		}, "Could not store chunk")
	}
}
