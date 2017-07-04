package chunk

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

// Downloader handles concurrent chunk downloads
type Downloader struct {
	Client *http.Client
}

// NewDownloader creates a new download manager
func NewDownloader(threads int, client *http.Client) (*Downloader, error) {
	manager := Downloader{
		Client: client,
	}

	return &manager, nil
}

// Download starts a new download request
func (d *Downloader) Download(req *Request) ([]byte, error) {
	return downloadFromAPI(d.Client, req, 0)
}

func downloadFromAPI(client *http.Client, request *Request, delay int64) ([]byte, error) {
	// sleep if request is throttled
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Second)
	}

	log.WithField("ObjectID", request.object.ObjectID).
		WithField("ObjectName", request.object.Name).
		WithField("OffsetStart", request.offsetStart).
		WithField("OffsetEnd", request.offsetEnd).
		WithField("Preload", request.preload).
		Debug("Requesting")
	req, err := http.NewRequest("GET", request.object.DownloadURL, nil)
	if nil != err {
		log.WithField("ObjectID", request.object.ObjectID).
			WithField("ObjectName", request.object.Name).
			WithField("OffsetStart", request.offsetStart).
			WithField("OffsetEnd", request.offsetEnd).
			WithField("Preload", request.preload).
			WithField("Error", err).
			Debug("Error")
		return nil, fmt.Errorf("Could not create request object %v (%v) from API", request.object.ObjectID, request.object.Name)
	}

	req.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", request.offsetStart, request.offsetEnd))

	log.WithField("ObjectID", request.object.ObjectID).
		WithField("ObjectName", request.object.Name).
		WithField("OffsetStart", request.offsetStart).
		WithField("OffsetEnd", request.offsetEnd).
		WithField("Preload", request.preload).
		WithField("HTTPRequest", req).
		Debug("Sending HTTP Request")

	res, err := client.Do(req)
	if nil != err {
		log.WithField("ObjectID", request.object.ObjectID).
			WithField("ObjectName", request.object.Name).
			WithField("OffsetStart", request.offsetStart).
			WithField("OffsetEnd", request.offsetEnd).
			WithField("Preload", request.preload).
			WithField("Error", err).
			Debug("Error")
		return nil, fmt.Errorf("Could not request object %v (%v) from API", request.object.ObjectID, request.object.Name)
	}
	defer res.Body.Close()
	reader := res.Body

	if res.StatusCode != 206 {
		if res.StatusCode != 403 && res.StatusCode != 500 {
			log.WithField("ObjectID", request.object.ObjectID).
				WithField("ObjectName", request.object.Name).
				WithField("OffsetStart", request.offsetStart).
				WithField("OffsetEnd", request.offsetEnd).
				WithField("Preload", request.preload).
				WithField("Request", req).
				WithField("Response", res).
				Debug("Wrong status code")
			return nil, fmt.Errorf("Wrong status code %v", res.StatusCode)
		}

		// throttle requests
		if delay > 8 {
			return nil, fmt.Errorf("Maximum throttle interval has been reached %v (%v)",
				request.object.ObjectID, request.object.Name)
		}
		bytes, err := ioutil.ReadAll(reader)
		if nil != err {
			log.WithField("ObjectID", request.object.ObjectID).
				WithField("ObjectName", request.object.Name).
				WithField("OffsetStart", request.offsetStart).
				WithField("OffsetEnd", request.offsetEnd).
				WithField("Preload", request.preload).
				WithField("Error", err).
				Debug("Error")
			return nil, fmt.Errorf("Could not read body of error")
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
			return downloadFromAPI(client, request, delay)
		}

		// return an error if other error occurred
		log.WithField("ObjectID", request.object.ObjectID).
			WithField("ObjectName", request.object.Name).
			WithField("OffsetStart", request.offsetStart).
			WithField("OffsetEnd", request.offsetEnd).
			WithField("Preload", request.preload).
			WithField("Body", body).
			WithField("StatusCode", res.StatusCode).
			Debug("Could not read object")
		return nil, fmt.Errorf("Could not read object %v (%v) / StatusCode: %v",
			request.object.ObjectID, request.object.Name, res.StatusCode)
	}

	bytes, err := ioutil.ReadAll(reader)
	if nil != err {
		log.WithField("ObjectID", request.object.ObjectID).
			WithField("ObjectName", request.object.Name).
			WithField("OffsetStart", request.offsetStart).
			WithField("OffsetEnd", request.offsetEnd).
			WithField("Preload", request.preload).
			WithField("Error", err).
			Debug("Error")
		return nil, fmt.Errorf("Could not read objects %v (%v) API response", request.object.ObjectID, request.object.Name)
	}

	return bytes, nil
}
