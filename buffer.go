package main

import (
	. "github.com/claudetech/loggo/default"
	"github.com/orcaman/concurrent-map"
)

var instances cmap.ConcurrentMap

func init() {
	instances = cmap.New()
}

// Buffer is a buffered stream
type Buffer struct {
	numberOfInstances int
	downloadManager   *DownloadManager
	object            *APIObject
}

// GetBufferInstance gets a singleton instance of buffer
func GetBufferInstance(downloadManager *DownloadManager, object *APIObject) (*Buffer, error) {
	if !instances.Has(object.ObjectID) {
		i, err := newBuffer(downloadManager, object)
		if nil != err {
			return nil, err
		}

		instances.Set(object.ObjectID, i)
	}

	instance, ok := instances.Get(object.ObjectID)
	// if buffer allocation failed due to race conditions it will try to fetch a new one
	if !ok {
		i, err := GetBufferInstance(downloadManager, object)
		if nil != err {
			return nil, err
		}
		instance = i
	}
	instance.(*Buffer).numberOfInstances++
	return instance.(*Buffer), nil
}

// NewBuffer creates a new buffer instance
func newBuffer(downloadManager *DownloadManager, object *APIObject) (*Buffer, error) {
	Log.Debugf("Creating buffer for object %v (%v)", object.ObjectID, object.Name)

	buffer := Buffer{
		numberOfInstances: 0,
		downloadManager:   downloadManager,
		object:            object,
	}

	return &buffer, nil
}

// Close all handles
func (b *Buffer) Close() error {
	b.numberOfInstances--
	if 0 == b.numberOfInstances {
		instances.Remove(b.object.ObjectID)
	}
	return nil
}

// ReadBytes on a specific location
func (b *Buffer) ReadBytes(offset, size int64) ([]byte, error) {
	return b.downloadManager.Download(b.object, offset, size)
}
