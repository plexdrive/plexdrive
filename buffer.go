package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"time"

	"strings"

	. "github.com/claudetech/loggo/default"
	"github.com/orcaman/concurrent-map"
)

var instances cmap.ConcurrentMap
var chunkPath string
var chunkSize int64
var chunkDirMaxSize int64

func init() {
	instances = cmap.New()
}

// Buffer is a buffered stream
type Buffer struct {
	numberOfInstances int
	client            *http.Client
	object            *APIObject
	tempDir           string
	preload           bool
	chunks            cmap.ConcurrentMap
}

// GetBufferInstance gets a singleton instance of buffer
func GetBufferInstance(client *http.Client, object *APIObject) (*Buffer, error) {
	if !instances.Has(object.ObjectID) {
		i, err := newBuffer(client, object)
		if nil != err {
			return nil, err
		}

		instances.Set(object.ObjectID, i)
	}

	instance, ok := instances.Get(object.ObjectID)
	// if buffer allocation failed due to race conditions it will try to fetch a new one
	if !ok {
		i, err := GetBufferInstance(client, object)
		if nil != err {
			return nil, err
		}
		instance = i
	}
	instance.(*Buffer).numberOfInstances++
	return instance.(*Buffer), nil
}

// SetChunkPath sets the global chunk path
func SetChunkPath(path string) {
	chunkPath = path
}

// SetChunkSize sets the global chunk size
func SetChunkSize(size int64) {
	chunkSize = size
}

// SetChunkDirMaxSize sets the maximum size of the chunk directory
func SetChunkDirMaxSize(size int64) {
	chunkDirMaxSize = size
}

// NewBuffer creates a new buffer instance
func newBuffer(client *http.Client, object *APIObject) (*Buffer, error) {
	Log.Infof("Starting playback of %v", object.Name)
	Log.Debugf("Creating buffer for object %v", object.ObjectID)

	tempDir := filepath.Join(chunkPath, object.ObjectID)
	if err := os.MkdirAll(tempDir, 0777); nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not create temp path for object %v", object.ObjectID)
	}

	if 0 == chunkSize {
		Log.Debugf("ChunkSize was 0, setting to default (5 MB)")
		chunkSize = 5 * 1024 * 1024
	}

	buffer := Buffer{
		numberOfInstances: 0,
		client:            client,
		object:            object,
		tempDir:           tempDir,
		preload:           true,
		chunks:            cmap.New(),
	}

	return &buffer, nil
}

// Close all handles
func (b *Buffer) Close() error {
	b.numberOfInstances--
	if 0 == b.numberOfInstances {
		Log.Infof("Stopping playback of %v", b.object.Name)
		Log.Debugf("Stop buffering for object %v", b.object.ObjectID)

		b.preload = false
		instances.Remove(b.object.ObjectID)
	}
	return nil
}

// ReadBytes on a specific location
func (b *Buffer) ReadBytes(start, size int64, delay int32) ([]byte, error) {
	fOffset := start % chunkSize
	offset := start - fOffset
	offsetEnd := offset + chunkSize

	Log.Tracef("Getting object %v - chunk %v - offset %v for %v bytes",
		b.object.ObjectID, strconv.Itoa(int(offset)), fOffset, size)

	if b.preload && uint64(offsetEnd) < b.object.Size {
		defer func() {
			go func() {
				preloadStart := strconv.Itoa(int(offsetEnd))
				if !b.chunks.Has(preloadStart) {
					b.chunks.Set(preloadStart, true)
					b.ReadBytes(offsetEnd, size, 0)
				}
			}()
		}()
	}

	filename := filepath.Join(b.tempDir, strconv.Itoa(int(offset)))
	if f, err := os.Open(filename); nil == err {
		defer f.Close()

		buf := make([]byte, size)
		if n, err := f.ReadAt(buf, fOffset); n > 0 && (nil == err || io.EOF == err) {
			Log.Tracef("Found file %s bytes %v - %v in cache", filename, offset, offsetEnd)

			// update the last modified time for files that are often in use
			if err := os.Chtimes(filename, time.Now(), time.Now()); nil != err {
				Log.Warningf("Could not update last modified time for %v", filename)
			}

			return buf[:size], nil
		}

		Log.Debugf("%v", err)
		Log.Debugf("Could not read file %s at %v", filename, fOffset)
	}

	if chunkDirMaxSize > 0 {
		go func() {
			if err := cleanChunkDir(chunkPath); nil != err {
				Log.Debugf("%v", err)
				Log.Warningf("Could not delete oldest chunk")
			}
		}()
	}

	// sleep if request is throttled
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Second)
	}

	Log.Debugf("Requesting object %v bytes %v - %v from API", b.object.ObjectID, offset, offsetEnd)
	req, err := http.NewRequest("GET", b.object.DownloadURL, nil)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not create request object %v from API", b.object.ObjectID)
	}

	req.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", offset, offsetEnd))

	Log.Tracef("Sending HTTP Request %v", req)

	res, err := b.client.Do(req)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not request object %v from API", b.object.ObjectID)
	}
	defer res.Body.Close()

	if res.StatusCode != 206 {
		if res.StatusCode != 403 {
			return nil, fmt.Errorf("Wrong status code %v", res)
		}

		// throttle requests
		if delay > 8 {
			return nil, fmt.Errorf("Maximum throttle interval has been reached")
		}
		bytes, err := ioutil.ReadAll(res.Body)
		if nil != err {
			Log.Debugf("%v", err)
			return nil, fmt.Errorf("Could not read body of 403 error")
		}
		body := string(bytes)
		if strings.Contains(body, "dailyLimitExceeded") ||
			strings.Contains(body, "userRateLimitExceeded") ||
			strings.Contains(body, "rateLimitExceeded") ||
			strings.Contains(body, "backendError") {
			if 0 == delay {
				delay = 1
			} else {
				delay = delay * 2
			}
			return b.ReadBytes(start, size, delay)
		}
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not read objects %v API response", b.object.ObjectID)
	}

	if err := ioutil.WriteFile(filename, bytes, 0777); nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not write chunk temp file %v", filename)
	}

	sOffset := int64(math.Min(float64(fOffset), float64(len(bytes))))
	eOffset := int64(math.Min(float64(fOffset+size), float64(len(bytes))))
	return bytes[sOffset:eOffset], nil
}

// cleanChunkDir checks if the chunk folder is grown to big and clears the oldest file if necessary
func cleanChunkDir(chunkPath string) error {
	chunkDirSize, err := dirSize(chunkPath)
	if nil != err {
		return err
	}

	if chunkDirSize+chunkSize*2 > chunkDirMaxSize {
		if err := deleteOldestFile(chunkPath); nil != err {
			return err
		}
	}

	return nil
}

// deleteOldestFile deletes the oldest file in the directory
func deleteOldestFile(path string) error {
	var fpath string
	lastMod := time.Now()

	err := filepath.Walk(path, func(file string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			modTime := info.ModTime()
			if modTime.Before(lastMod) {
				lastMod = modTime
				fpath = file
			}
		}
		return err
	})

	os.Remove(fpath)

	return err
}

// dirSize gets the total directory size
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if nil != err && nil != info && !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
