package main

import (
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/orcaman/concurrent-map"
)

var instances cmap.ConcurrentMap

func init() {
	instances = cmap.New()
}

// Buffer is a buffered stream
type Buffer struct {
	numberOfInstances int
	client            *http.Client
	object            *APIObject
	tempDir           string
	chunkSize         int64
	preload           bool
	chunkDir          string
}

// GetBufferInstance gets a singleton instance of buffer
func GetBufferInstance(client *http.Client, object *APIObject, chunkSize int64, chunkDir string) (*Buffer, error) {
	if !instances.Has(object.ID) {
		i, err := newBuffer(client, object, chunkSize, chunkDir)
		if nil != err {
			return nil, err
		}

		instances.Set(object.ID, i)
	}

	instance, _ := instances.Get(object.ID)
	instance.(*Buffer).numberOfInstances++
	return instance.(*Buffer), nil
}

// NewBuffer creates a new buffer instance
func newBuffer(client *http.Client, object *APIObject, chunkSize int64, chunkDir string) (*Buffer, error) {
	tempDir := filepath.Join(chunkDir, object.ID)
	if err := os.MkdirAll(tempDir, 0777); nil != err {
		return nil, err
	}

	if chunkSize == 0 {
		chunkSize = 5 * 1024 * 1024
	}

	buffer := Buffer{
		numberOfInstances: 0,
		client:            client,
		object:            object,
		tempDir:           tempDir,
		chunkSize:         chunkSize,
		preload:           true,
	}

	return &buffer, nil
}

// Close all handles
func (b *Buffer) Close() error {
	b.numberOfInstances--
	if b.numberOfInstances == 0 {
		b.preload = false
		instances.Remove(b.object.ID)
	}
	return nil
}

// ReadBytes on a specific location
func (b *Buffer) ReadBytes(start, size int64) ([]byte, error) {
	fOffset := start % b.chunkSize
	offset := start - fOffset
	offsetEnd := offset + b.chunkSize

	filename := filepath.Join(b.tempDir, strconv.Itoa(int(offset)))

	if f, err := os.Open(filename); nil == err {
		defer f.Close()
		buf := make([]byte, size)
		if _, err := f.ReadAt(buf, fOffset); nil == err {
			return buf[:size], nil
		}
	}

	req, err := http.NewRequest("GET", b.object.DownloadURL, nil)
	if nil != err {
		return nil, err
	}

	req.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", offset, offsetEnd))

	res, err := b.client.Do(req)
	if nil != err {
		return nil, err
	}

	if res.StatusCode != 206 {
		return nil, fmt.Errorf("Wrong status code %v", res)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if nil != err {
		return nil, err
	}

	f, err := os.Create(filename)
	if nil != err {
		return nil, err
	}
	defer f.Close()

	_, err = f.Write(bytes)
	if nil != err {
		return nil, err
	}

	if b.preload && uint64(offsetEnd) < b.object.Size {
		go func() {
			b.ReadBytes(offsetEnd+1, size)
		}()
	}

	return bytes[fOffset:int64(math.Min(float64(start+size), float64(len(bytes))))], nil
}
