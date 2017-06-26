package chunk

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	. "github.com/claudetech/loggo/default"
)

// Downloader handles concurrent chunk downloads
type Downloader struct {
	Client  *http.Client
	hpQueue chan *download
	lpQueue chan *download
}

type download struct {
	request  *Request
	response chan *response
}

type response struct {
	bytes []byte
	err   error
}

// NewDownloader creates a new download manager
func NewDownloader(threads int, client *http.Client) (*Downloader, error) {
	manager := Downloader{
		Client:  client,
		hpQueue: make(chan *download, 100),
		lpQueue: make(chan *download, 100),
	}

	if threads < 1 {
		return nil, fmt.Errorf("Number of threads for download manager must not be < 1")
	}

	for i := 0; i < threads/2; i++ {
		go manager.thread()
	}

	return &manager, nil
}

func (d *Downloader) Download(req *Request) ([]byte, error) {
	rc := make(chan *response)

	request := download{
		request:  req,
		response: rc,
	}

	if !req.preload {
		d.hpQueue <- &request
	} else {
		d.lpQueue <- &request
	}

	res := <-rc
	return res.bytes, res.err
}

func (d *Downloader) thread() {
	for {
		select {
		case download := <-d.hpQueue:
			bytes, err := downloadFromAPI(d.Client, download.request, 0)
			download.response <- &response{
				bytes: bytes,
				err:   err,
			}
			close(download.response)
			break
		case download := <-d.lpQueue:
			bytes, err := downloadFromAPI(d.Client, download.request, 0)
			download.response <- &response{
				bytes: bytes,
				err:   err,
			}
			close(download.response)
			break
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func downloadFromAPI(client *http.Client, request *Request, delay int64) ([]byte, error) {
	// sleep if request is throttled
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Second)
	}

	Log.Debugf("Requesting object %v (%v) bytes %v - %v from API (preload: %v)",
		request.object.ObjectID, request.object.Name, request.offsetStart, request.offsetEnd, request.preload)
	req, err := http.NewRequest("GET", request.object.DownloadURL, nil)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not create request object %v (%v) from API", request.object.ObjectID, request.object.Name)
	}

	req.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", request.offsetStart, request.offsetEnd))

	Log.Tracef("Sending HTTP Request %v", req)

	res, err := client.Do(req)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not request object %v (%v) from API", request.object.ObjectID, request.object.Name)
	}
	defer res.Body.Close()
	reader := res.Body

	if res.StatusCode != 206 {
		if res.StatusCode != 403 {
			Log.Debugf("Request\n----------\n%v\n----------\n", req)
			Log.Debugf("Response\n----------\n%v\n----------\n", res)
			return nil, fmt.Errorf("Wrong status code %v", res.StatusCode)
		}

		// throttle requests
		if delay > 8 {
			return nil, fmt.Errorf("Maximum throttle interval has been reached")
		}
		bytes, err := ioutil.ReadAll(reader)
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
			return downloadFromAPI(client, request, delay)
		}

		// return an error if other 403 error occurred
		Log.Debugf("%v", body)
		return nil, fmt.Errorf("Could not read object %v (%v) / StatusCode: %v",
			request.object.ObjectID, request.object.Name, res.StatusCode)
	}

	bytes, err := ioutil.ReadAll(reader)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not read objects %v (%v) API response", request.object.ObjectID, request.object.Name)
	}

	return bytes, nil
}
