package chunk

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/claudetech/loggo/default"
	"github.com/plexdrive/plexdrive/drive"
)

// Downloader handles concurrent chunk downloads
type Downloader struct {
	Client     *drive.Client
	BufferSize int64
	queue      chan *Request
	callbacks  map[string][]DownloadCallback
	lock       sync.Mutex
	storage    *Storage
}

type DownloadCallback func(error, []byte)

// NewDownloader creates a new download manager
func NewDownloader(threads int, client *drive.Client, storage *Storage, bufferSize int64) (*Downloader, error) {
	manager := Downloader{
		Client:     client,
		BufferSize: bufferSize,
		queue:      make(chan *Request, 100),
		callbacks:  make(map[string][]DownloadCallback, 100),
		storage:    storage,
	}

	for i := 0; i < threads; i++ {
		go manager.thread()
	}

	return &manager, nil
}

// Download starts a new download request
func (d *Downloader) Download(req *Request, callback DownloadCallback) {
	d.lock.Lock()
	callbacks, exists := d.callbacks[req.id]
	if nil != callback {
		d.callbacks[req.id] = append(callbacks, callback)
	} else if !exists {
		d.callbacks[req.id] = callbacks
	}
	if !exists {
		d.queue <- req
	}
	d.lock.Unlock()
}

func (d *Downloader) thread() {
	buffer := make([]byte, d.BufferSize)
	for {
		req := <-d.queue
		d.download(d.Client.GetNativeClient(), req, buffer)
	}
}

func (d *Downloader) download(client *http.Client, req *Request, buffer []byte) {
	Log.Debugf("Starting download %v (preload: %v)", req.id, req.preload)
	err := downloadFromAPI(client, req, buffer, 0)

	d.lock.Lock()
	callbacks := d.callbacks[req.id]
	for _, callback := range callbacks {
		callback(err, buffer)
	}
	delete(d.callbacks, req.id)
	d.lock.Unlock()

	if nil != err {
		return
	}

	if err := d.storage.Store(req.id, buffer); nil != err {
		Log.Warningf("Could not store chunk %v: %v", req.id, err)
	}
}

func downloadFromAPI(client *http.Client, request *Request, buffer []byte, delay int64) error {
	// sleep if request is throttled
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Second)
	}

	downloadURL := request.object.DownloadURL
	if request.acknowledgeAbuse {
		downloadURL += "&acknowledgeAbuse=true"
	}
	req, err := http.NewRequest("GET", downloadURL, nil)
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not create request object %v (%v) from API", request.object.ObjectID, request.object.Name)
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", request.offsetStart, request.offsetEnd-1))

	Log.Tracef("Sending HTTP Request %v", req)

	res, err := client.Do(req)
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not request object %v (%v) from API", request.object.ObjectID, request.object.Name)
	}
	defer res.Body.Close()
	reader := res.Body

	if res.StatusCode != 206 {
		if res.StatusCode != 403 && res.StatusCode != 500 {
			Log.Debugf("Request\n----------\n%v\n----------\n", req)
			Log.Debugf("Response\n----------\n%v\n----------\n", res)
			return fmt.Errorf("Wrong status code %v for %v", res.StatusCode, request.object)
		}

		// throttle requests
		if delay > 8 {
			return fmt.Errorf("Maximum throttle interval has been reached")
		}
		bytes, err := ioutil.ReadAll(reader)
		if nil != err {
			Log.Debugf("%v", err)
			return fmt.Errorf("Could not read body of error")
		}
		body := string(bytes)
		if strings.Contains(body, "dailyLimitExceeded") ||
			strings.Contains(body, "userRateLimitExceeded") ||
			strings.Contains(body, "rateLimitExceeded") ||
			strings.Contains(body, "backendError") ||
			strings.Contains(body, "internalError") {
			if 0 == delay {
				delay = 1
			} else {
				delay = delay * 2
			}
			return downloadFromAPI(client, request, buffer, delay)
		}

		// return an error if other error occurred
		Log.Debugf("%v", body)
		return fmt.Errorf("Could not read object %v (%v) / StatusCode: %v",
			request.object.ObjectID, request.object.Name, res.StatusCode)
	}

	if res.ContentLength == -1 {
		return fmt.Errorf("Missing Content-Length header in response")
	}

	n, err := io.ReadFull(reader, buffer[:res.ContentLength:cap(buffer)])
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not read objects %v (%v) API response", request.object.ObjectID, request.object.Name)
	}
	Log.Debugf("Downloaded %v bytes of %v (%v)", n, request.object.ObjectID, request.object.Name)

	return nil
}
