package chunk

import (
	"fmt"

	"net/http"

	"os"

	. "github.com/claudetech/loggo/default"

	"sync"

	"math"

	"github.com/dweidenfeld/plexdrive/drive"
)

// Manager manages chunks on disk
type Manager struct {
	ChunkPath  string
	ChunkSize  int64
	LoadAhead  int
	downloader *Downloader
	queue      chan *Request
	chunks     map[string]bool
	chunksLock sync.Mutex
	storage    *Storage
}

type Request struct {
	id          string
	object      *drive.APIObject
	preload     bool
	offsetStart int64
	offsetEnd   int64
}

// NewManager creates a new chunk manager
func NewManager(chunkPath string, chunkSize int64, loadAhead, threads int, client *http.Client) (*Manager, error) {
	if "" == chunkPath {
		return nil, fmt.Errorf("Path to chunk file must not be empty")
	}
	if chunkSize < 4096 {
		return nil, fmt.Errorf("Chunk size must not be < 4096")
	}
	if chunkSize%1024 != 0 {
		return nil, fmt.Errorf("Chunk size must be divideable by 1024")
	}

	downloader, err := NewDownloader(threads, client)
	if nil != err {
		return nil, err
	}

	manager := Manager{
		ChunkPath:  chunkPath,
		ChunkSize:  chunkSize,
		LoadAhead:  loadAhead,
		downloader: downloader,
		queue:      make(chan *Request, 50),
		chunks:     make(map[string]bool),
		storage:    NewStorage(),
	}

	if err := os.RemoveAll(chunkPath); nil != err {
		return nil, fmt.Errorf("Could not clear old chunks from disk")
	}

	for i := 0; i < threads/2; i++ {
		go manager.thread()
	}

	return &manager, nil
}

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

	return m.getChunkBlocking(id, chunkOffset, size)
}

func (m *Manager) thread() {
	for {
		m.checkChunk(<-m.queue)
	}
}

func (m *Manager) getChunkBlocking(id string, chunkOffset, size int64) ([]byte, error) {
	bytes, err := m.storage.Get(id)
	if nil != err {
		return nil, err
	}

	sOffset := int64(math.Min(float64(len(bytes)), float64(chunkOffset)))
	eOffset := int64(math.Min(float64(len(bytes)), float64(chunkOffset+size)))
	return bytes[sOffset:eOffset], nil
}

func (m *Manager) checkChunk(req *Request) {
	m.chunksLock.Lock()
	if _, exists := m.chunks[req.id]; exists {
		m.chunksLock.Unlock()
		return
	}
	m.chunks[req.id] = true
	m.chunksLock.Unlock()

	bytes, err := m.downloader.Download(req)
	if nil != err {
		Log.Warningf("%v", err)
	}

	if err := m.storage.Store(req.id, bytes); nil != err {
		Log.Warningf("%v", err)
	}
}

// func (m *Manager) PreloadChunks(req *ChunkRequest) {
// 	fOffset := req.Offset % m.ChunkSize
// 	offsetStart := req.Offset - fOffset

// 	for i := m.ChunkSize; i < (m.ChunkSize * int64(m.LoadAhead+1)); i += m.ChunkSize {
// 		m.RequestChunk(&ChunkRequest{
// 			Object:  req.Object,
// 			Offset:  offsetStart + i,
// 			Size:    req.Size,
// 			Preload: true,
// 		})
// 	}
// }

// func (m *Manager) storingThread() {
// 	for {
// 		queueItem := <-m.storeQueue
// 		m.storeChunkToDisk(queueItem.ChunkRequest, queueItem.ChunkResponse)
// 	}
// }

// func (m *Manager) loadChunkFromRAM(req *ChunkRequest) *ChunkResponse {
// 	bytes, exists := m.chunks[req.id]
// 	if !exists {
// 		return &ChunkResponse{
// 			Error: fmt.Errorf("Could not find chunk %v in memory", req.id),
// 		}
// 	}

// 	eOffset := int64(math.Min(float64(req.Size), float64(len(bytes))))
// 	return &ChunkResponse{
// 		Bytes: bytes[:eOffset],
// 	}
// }

// func (m *Manager) storeChunkInRAM(req *ChunkRequest, res *ChunkResponse) {
// 	m.chunkLock.Lock()
// 	m.chunks[req.id] = res.Bytes
// 	m.chunkLock.Unlock()
// }

// func (m *Manager) loadChunkFromDisk(req *ChunkRequest) *ChunkResponse {
// 	chunkDir := filepath.Join(m.ChunkPath, req.Object.ObjectID)
// 	filename := filepath.Join(chunkDir, strconv.Itoa(int(req.offsetStart)))

// 	f, err := os.Open(filename)
// 	if nil != err {
// 		Log.Tracef("%v", err)
// 		return &ChunkResponse{
// 			Error: fmt.Errorf("Could not open file %v", filename),
// 		}
// 	}
// 	defer f.Close()

// 	buf := make([]byte, req.Size)
// 	n, err := f.ReadAt(buf, req.fOffset)
// 	if n > 0 && (nil == err || io.EOF == err || io.ErrUnexpectedEOF == err) {
// 		Log.Tracef("Found file %s bytes %v - %v in cache", filename, req.offsetStart, req.offsetEnd)

// 		// update the last modified time for files that are often in use
// 		if err := os.Chtimes(filename, time.Now(), time.Now()); nil != err {
// 			Log.Warningf("Could not update last modified time for %v", filename)
// 		}

// 		eOffset := int64(math.Min(float64(req.Size), float64(len(buf))))
// 		return &ChunkResponse{
// 			Bytes: buf[:eOffset],
// 		}
// 	}

// 	Log.Tracef("%v", err)
// 	return &ChunkResponse{
// 		Error: fmt.Errorf("Could not read file %s at %v", filename, req.fOffset),
// 	}
// }

// func (m *Manager) storeChunkToDisk(req *ChunkRequest, res *ChunkResponse) {
// 	chunkDir := filepath.Join(m.ChunkPath, req.Object.ObjectID)
// 	filename := filepath.Join(chunkDir, strconv.Itoa(int(req.offsetStart)))

// 	if _, err := os.Stat(chunkDir); os.IsNotExist(err) {
// 		if err := os.MkdirAll(chunkDir, 0777); nil != err {
// 			Log.Debugf("%v", err)
// 			Log.Warningf("Could not create chunk temp path %v", chunkDir)
// 		}
// 	}

// 	if _, err := os.Stat(filename); os.IsNotExist(err) {
// 		if err := ioutil.WriteFile(filename, res.Bytes, 0777); nil != err {
// 			Log.Debugf("%v", err)
// 			Log.Warningf("Could not write chunk temp file %v", filename)
// 		}
// 	}

// 	m.chunkLock.Lock()
// 	delete(m.chunks, req.id)
// 	m.chunkLock.Unlock()
// }
