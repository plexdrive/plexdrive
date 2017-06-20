package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	. "github.com/claudetech/loggo/default"
)

// DownloadManager handles concurrent chunk downloads
type DownloadManager struct {
	Client *http.Client
	Queue  *Queue
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(threadCount int, client *http.Client) (*DownloadManager, error) {
	manager := DownloadManager{
		Client: client,
		Queue:  NewQueue(),
	}

	if threadCount < 1 {
		return nil, fmt.Errorf("Number of threads for download manager must not be < 1")
	}

	for i := 0; i < threadCount; i++ {
		go manager.downloadThread()
	}

	return &manager, nil
}

func (m *DownloadManager) downloadThread() {
	for {
		m.Download(m.Queue.Pop())
	}
}

func (m *DownloadManager) RequestChunk(req *ChunkRequest) *ChunkResponse {
	if req.Preload {
		return <-m.Queue.PushRight(req)
	}
	return <-m.Queue.PushLeft(req)
}

func (m *DownloadManager) Download(req *ChunkRequest, res chan *ChunkResponse) {
	res <- downloadFromAPI(m.Client, req, 0)
}

func downloadFromAPI(client *http.Client, request *ChunkRequest, delay int64) *ChunkResponse {
	// sleep if request is throttled
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Second)
	}

	Log.Debugf("Requesting object %v (%v) bytes %v - %v from API (preload: %v)",
		request.Object.ObjectID, request.Object.Name, request.offsetStart, request.offsetEnd, request.Preload)
	req, err := http.NewRequest("GET", request.Object.DownloadURL, nil)
	if nil != err {
		Log.Debugf("%v", err)
		return &ChunkResponse{
			Error: fmt.Errorf("Could not create request object %v (%v) from API", request.Object.ObjectID, request.Object.Name),
		}
	}

	req.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", request.offsetStart, request.offsetEnd))

	Log.Tracef("Sending HTTP Request %v", req)

	res, err := client.Do(req)
	if nil != err {
		Log.Debugf("%v", err)
		return &ChunkResponse{
			Error: fmt.Errorf("Could not request object %v (%v) from API", request.Object.ObjectID, request.Object.Name),
		}
	}
	defer res.Body.Close()
	reader := res.Body

	if res.StatusCode != 206 {
		if res.StatusCode != 403 {
			Log.Debugf("Request\n----------\n%v\n----------\n", req)
			Log.Debugf("Response\n----------\n%v\n----------\n", res)
			return &ChunkResponse{
				Error: fmt.Errorf("Wrong status code %v", res.StatusCode),
			}
		}

		// throttle requests
		if delay > 8 {
			return &ChunkResponse{
				Error: fmt.Errorf("Maximum throttle interval has been reached"),
			}
		}
		bytes, err := ioutil.ReadAll(reader)
		if nil != err {
			Log.Debugf("%v", err)
			return &ChunkResponse{
				Error: fmt.Errorf("Could not read body of 403 error"),
			}
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
			return downloadFromAPI(client, request, delay)
		}

		// return an error if other 403 error occurred
		Log.Debugf("%v", body)
		return &ChunkResponse{
			Error: fmt.Errorf("Could not read object %v (%v) / StatusCode: %v",
				request.Object.ObjectID, request.Object.Name, res.StatusCode),
		}
	}

	bytes, err := ioutil.ReadAll(reader)
	if nil != err {
		Log.Debugf("%v", err)
		return &ChunkResponse{
			Error: fmt.Errorf("Could not read objects %v (%v) API response", request.Object.ObjectID, request.Object.Name),
		}
	}

	return &ChunkResponse{
		Bytes: bytes,
	}
}
