package chunk

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	. "github.com/claudetech/loggo/default"
	"github.com/dweidenfeld/plexdrive/drive"
)

// Manager manages chunks on disk
type Manager struct {
	ChunkPath  string
	ChunkSize  int64
	LoadAhead  int
	downloader *Downloader
	chunks     map[string][]byte
	chunkLock  sync.Mutex
	storeQueue chan ChunkQueueItem
}

type ChunkRequest struct {
	Object      *drive.APIObject
	Offset      int64
	Size        int64
	Preload     bool
	id          string
	fOffset     int64
	offsetStart int64
	offsetEnd   int64
}

type ChunkResponse struct {
	Error error
	Bytes []byte
}

type ChunkQueueItem struct {
	*ChunkRequest
	*ChunkResponse
}

// NewManager creates a new chunk manager
func NewManager(downloader *Downloader, chunkPath string, chunkSize int64, loadAhead int) (*Manager, error) {
	if "" == chunkPath {
		return nil, fmt.Errorf("Path to chunk file must not be empty")
	}
	if chunkSize < 4096 {
		return nil, fmt.Errorf("Chunk size must not be < 4096")
	}
	if chunkSize%1024 != 0 {
		return nil, fmt.Errorf("Chunk size must be divideable by 1024")
	}

	manager := Manager{
		ChunkPath:  chunkPath,
		ChunkSize:  chunkSize,
		LoadAhead:  loadAhead,
		downloader: downloader,
		chunks:     make(map[string][]byte),
		storeQueue: make(chan ChunkQueueItem, 100),
	}

	go manager.storingThread()

	return &manager, nil
}

func (m *Manager) RequestChunk(req *ChunkRequest) <-chan *ChunkResponse {
	res := make(chan *ChunkResponse)

	go func() {
		defer close(res)

		req.fOffset = req.Offset % m.ChunkSize
		req.offsetStart = req.Offset - req.fOffset
		req.offsetEnd = req.offsetStart + m.ChunkSize
		req.id = fmt.Sprintf("%v:%v", req.Object.ObjectID, req.offsetStart)

		ramRes := m.loadChunkFromRAM(req)
		if nil != ramRes.Error {
			Log.Tracef("%v", ramRes.Error)
		} else {
			res <- ramRes
			return
		}

		diskRes := m.loadChunkFromDisk(req)
		if nil != diskRes.Error {
			Log.Tracef("%v", diskRes.Error)
		} else {
			res <- diskRes
			return
		}

		apiRes := m.downloader.RequestChunk(req)

		if nil == apiRes.Error {
			sOffset := int64(math.Min(float64(req.fOffset), float64(len(apiRes.Bytes))))
			eOffset := int64(math.Min(float64(req.fOffset+req.Size), float64(len(apiRes.Bytes))))
			res <- &ChunkResponse{
				Bytes: apiRes.Bytes[sOffset:eOffset],
			}

			m.storeChunkInRAM(req, apiRes)
			m.storeQueue <- ChunkQueueItem{
				ChunkRequest:  req,
				ChunkResponse: apiRes,
			}
		} else {
			res <- apiRes
		}
	}()

	return res
}

func (m *Manager) PreloadChunks(req *ChunkRequest) {
	fOffset := req.Offset % m.ChunkSize
	offsetStart := req.Offset - fOffset

	for i := m.ChunkSize; i < (m.ChunkSize * int64(m.LoadAhead+1)); i += m.ChunkSize {
		m.RequestChunk(&ChunkRequest{
			Object:  req.Object,
			Offset:  offsetStart + i,
			Size:    req.Size,
			Preload: true,
		})
	}
}

func (m *Manager) storingThread() {
	for {
		queueItem := <-m.storeQueue
		m.storeChunkToDisk(queueItem.ChunkRequest, queueItem.ChunkResponse)
	}
}

func (m *Manager) loadChunkFromRAM(req *ChunkRequest) *ChunkResponse {
	bytes, exists := m.chunks[req.id]
	if !exists {
		return &ChunkResponse{
			Error: fmt.Errorf("Could not find chunk %v in memory", req.id),
		}
	}

	eOffset := int64(math.Min(float64(req.Size), float64(len(bytes))))
	return &ChunkResponse{
		Bytes: bytes[:eOffset],
	}
}

func (m *Manager) storeChunkInRAM(req *ChunkRequest, res *ChunkResponse) {
	m.chunkLock.Lock()
	m.chunks[req.id] = res.Bytes
	m.chunkLock.Unlock()
}

func (m *Manager) loadChunkFromDisk(req *ChunkRequest) *ChunkResponse {
	chunkDir := filepath.Join(m.ChunkPath, req.Object.ObjectID)
	filename := filepath.Join(chunkDir, strconv.Itoa(int(req.offsetStart)))

	f, err := os.Open(filename)
	if nil != err {
		Log.Tracef("%v", err)
		return &ChunkResponse{
			Error: fmt.Errorf("Could not open file %v", filename),
		}
	}
	defer f.Close()

	buf := make([]byte, req.Size)
	n, err := f.ReadAt(buf, req.fOffset)
	if n > 0 && (nil == err || io.EOF == err || io.ErrUnexpectedEOF == err) {
		Log.Tracef("Found file %s bytes %v - %v in cache", filename, req.offsetStart, req.offsetEnd)

		// update the last modified time for files that are often in use
		if err := os.Chtimes(filename, time.Now(), time.Now()); nil != err {
			Log.Warningf("Could not update last modified time for %v", filename)
		}

		eOffset := int64(math.Min(float64(req.Size), float64(len(buf))))
		return &ChunkResponse{
			Bytes: buf[:eOffset],
		}
	}

	Log.Tracef("%v", err)
	return &ChunkResponse{
		Error: fmt.Errorf("Could not read file %s at %v", filename, req.fOffset),
	}
}

func (m *Manager) storeChunkToDisk(req *ChunkRequest, res *ChunkResponse) {
	chunkDir := filepath.Join(m.ChunkPath, req.Object.ObjectID)
	filename := filepath.Join(chunkDir, strconv.Itoa(int(req.offsetStart)))

	if _, err := os.Stat(chunkDir); os.IsNotExist(err) {
		if err := os.MkdirAll(chunkDir, 0777); nil != err {
			Log.Debugf("%v", err)
			Log.Warningf("Could not create chunk temp path %v", chunkDir)
		}
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if err := ioutil.WriteFile(filename, res.Bytes, 0777); nil != err {
			Log.Debugf("%v", err)
			Log.Warningf("Could not write chunk temp file %v", filename)
		}
	}

	m.chunkLock.Lock()
	delete(m.chunks, req.id)
	m.chunkLock.Unlock()
}
