package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/claudetech/loggo/default"
)

// ChunkManager manages chunks on disk
type ChunkManager struct {
	ChunkPath string
	ChunkSize int64
}

// NewChunkManager creates a new chunk manager
func NewChunkManager(chunkPath string, chunkSize int64) (*ChunkManager, error) {
	if "" == chunkPath {
		return nil, fmt.Errorf("Path to chunk file must not be empty")
	}
	if chunkSize < 4096 {
		return nil, fmt.Errorf("Chunk size must not be < 4096")
	}
	if chunkSize%1024 != 0 {
		return nil, fmt.Errorf("Chunk size must be divideable by 1024")
	}

	manager := ChunkManager{
		ChunkPath: chunkPath,
		ChunkSize: chunkSize,
	}

	return &manager, nil
}

// GetChunk gets the partial content of a chunk
func (m *ChunkManager) GetChunk(object *APIObject, offset, size int64) ([]byte, error) {
	fOffset := offset % m.ChunkSize
	offsetStart := offset - fOffset
	offsetEnd := offsetStart + m.ChunkSize

	chunkDir := filepath.Join(m.ChunkPath, object.ObjectID)
	filename := filepath.Join(chunkDir, strconv.Itoa(int(offsetStart)))

	f, err := os.Open(filename)
	if nil != err {
		Log.Tracef("%v", err)
		return nil, fmt.Errorf("Could not open file %v", filename)
	}
	defer f.Close()

	buf := make([]byte, size)
	n, err := f.ReadAt(buf, fOffset)
	if n > 0 && (nil == err || io.EOF == err || io.ErrUnexpectedEOF == err) {
		Log.Tracef("Found file %s bytes %v - %v in cache", filename, offsetStart, offsetEnd)

		// update the last modified time for files that are often in use
		if err := os.Chtimes(filename, time.Now(), time.Now()); nil != err {
			Log.Warningf("Could not update last modified time for %v", filename)
		}

		eOffset := int64(math.Min(float64(size), float64(len(buf))))
		return buf[:eOffset], nil
	}

	Log.Tracef("%v", err)
	return nil, fmt.Errorf("Could not read file %s at %v", filename, fOffset)
}

// StoreChunk stores a chunk on disk
func (m *ChunkManager) StoreChunk(object *APIObject, offset int64, content []byte) {
	go func() {
		fOffset := offset % m.ChunkSize
		offsetStart := offset - fOffset

		chunkDir := filepath.Join(m.ChunkPath, object.ObjectID)
		filename := filepath.Join(chunkDir, strconv.Itoa(int(offsetStart)))

		if _, err := os.Stat(chunkDir); os.IsNotExist(err) {
			if err := os.MkdirAll(chunkDir, 0777); nil != err {
				Log.Debugf("%v", err)
				Log.Warningf("Could not create chunk temp path %v", chunkDir)
			}
		}

		if _, err := os.Stat(filename); os.IsNotExist(err) {
			if err := ioutil.WriteFile(filename, content, 0777); nil != err {
				Log.Debugf("%v", err)
				Log.Warningf("Could not write chunk temp file %v", filename)
			}
		}
	}()
}
