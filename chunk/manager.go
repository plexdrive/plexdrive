package chunk

import (
	"fmt"

	. "github.com/claudetech/loggo/default"

	"github.com/plexdrive/plexdrive/drive"
)

// Manager manages chunks on disk
type Manager struct {
	ChunkSize  int64
	LoadAhead  int
	downloader *Downloader
	storage    *Storage
	queue      chan *QueueEntry
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
	chunkSize int64,
	loadAhead,
	checkThreads int,
	loadThreads int,
	client *drive.Client,
	maxChunks int) (*Manager, error) {

	if chunkSize < 4096 {
		return nil, fmt.Errorf("Chunk size must not be < 4096")
	}
	if chunkSize%1024 != 0 {
		return nil, fmt.Errorf("Chunk size must be divideable by 1024")
	}
	if maxChunks < 2 || maxChunks < loadAhead {
		return nil, fmt.Errorf("max-chunks must be greater than 2 and bigger than the load ahead value")
	}

	downloader, err := NewDownloader(loadThreads, client)
	if nil != err {
		return nil, err
	}

	manager := Manager{
		ChunkSize:  chunkSize,
		LoadAhead:  loadAhead,
		downloader: downloader,
		storage:    NewStorage(chunkSize, maxChunks),
		queue:      make(chan *QueueEntry, 100),
	}

	if err := manager.storage.Clear(); nil != err {
		return nil, err
	}

	for i := 0; i < checkThreads; i++ {
		go manager.thread()
	}

	return &manager, nil
}

// GetChunk loads one chunk and starts the preload for the next chunks
func (m *Manager) GetChunk(object *drive.APIObject, offset, size int64) ([]byte, error) {
	maxOffset := int64(object.Size)
	if offset > maxOffset {
		return nil, fmt.Errorf("Tried to read past EOF of %v at offset %v", object.ObjectID, offset)
	}
	if offset+size > maxOffset {
		size = int64(object.Size) - offset
		Log.Debugf("Adjusting read past EOF of %v at offset %v to %v bytes", object.ObjectID, offset, size)
	}

	data := make([]byte, size, size)

	// Handle unaligned requests across chunk boundaries (Direct-IO)
	for read := int64(0); read < size; {
		response := make(chan Response)

		m.requestChunk(object, offset+read, size-read, response)

		res := <-response
		if nil != res.Error {
			return nil, res.Error
		}

		n := copy(data[read:], res.Bytes)

		if n == 0 {
			return nil, fmt.Errorf("Short read of %v at offset %v only %v / %v bytes", object.ObjectID, offset, read, size)
		}

		read += int64(n)
	}
	return data, nil
}

func (m *Manager) requestChunk(object *drive.APIObject, offset, size int64, response chan Response) {
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

func (m *Manager) thread() {
	for {
		queueEntry := <-m.queue
		m.checkChunk(queueEntry.request, queueEntry.response)
	}
}

func (m *Manager) checkChunk(req *Request, response chan Response) {
	if bytes := m.storage.Load(req.id); nil != bytes {
		if nil != response {
			response <- Response{
				Bytes: adjustResponseChunk(req, bytes),
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
				Bytes: adjustResponseChunk(req, bytes),
			}
			close(response)
		}

		if err := m.storage.Store(req.id, bytes); nil != err {
			Log.Warningf("Coult not store chunk %v", req.id)
		}
	})
}

func adjustResponseChunk(req *Request, bytes []byte) []byte {
	bytesLen := int64(len(bytes))
	sOffset := min(req.chunkOffset, bytesLen)
	eOffset := min(req.chunkOffsetEnd, bytesLen)
	return bytes[sOffset:eOffset]
}

func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}
