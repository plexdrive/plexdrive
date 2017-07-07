package chunk

import (
	"fmt"

	"net/http"

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
	queue          chan *Request
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
		queue:          make(chan *Request, 500),
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
			m.queue <- &Request{
				id:          id,
				object:      object,
				offsetStart: aheadOffsetStart,
				offsetEnd:   aheadOffsetEnd,
				preload:     true,
			}
		}
	}

	bytes, err := m.storage.Get(id, chunkOffset, size, m.Timeout)
	retryCount := 0
	for err == ErrTimeout && retryCount < m.TimeoutRetries {
		Log.Warningf("Timeout while requesting chunk %v. Retrying (%v / %v)", id, (retryCount + 1), m.TimeoutRetries)
		bytes, err = m.storage.Get(id, chunkOffset, size, m.Timeout)
		retryCount++
	}
	return bytes, err
}

func (m *Manager) thread(threadID int) {
	for {
		m.checkChunk(<-m.queue, threadID)
	}
}

func (m *Manager) checkChunk(req *Request, threadID int) {
	if m.storage.ExistsOrCreate(req.id) {
		return
	}

	Log.Debugf("Requesting object %v (%v) bytes %v - %v from API (preload: %v | thread: %v)",
		req.object.ObjectID, req.object.Name, req.offsetStart, req.offsetEnd, req.preload, threadID)

	bytes, err := m.downloader.Download(req)
	if nil != err {
		Log.Warningf("%v", err)
		m.storage.Error(req.id, err)
	}

	if err := m.storage.Store(req.id, bytes); nil != err {
		Log.Warningf("%v", err)
	}
}
