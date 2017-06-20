package main

import (
	"sync"
)

type Queue struct {
	hpQueue   chan *DownloadRequest
	lpQueue   chan *DownloadRequest
	activeIDs map[string]bool
	lock      sync.Mutex
}

func NewQueue() *Queue {
	q := &Queue{
		hpQueue:   make(chan *DownloadRequest, 999),
		lpQueue:   make(chan *DownloadRequest, 999),
		activeIDs: make(map[string]bool, 999),
	}
	return q
}

func (q *Queue) Put(request *DownloadRequest) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if _, exists := q.activeIDs[request.chunkID]; exists {
		return
	}

	if request.highPrio {
		q.hpQueue <- request
	} else {
		q.lpQueue <- request
	}

	q.activeIDs[request.chunkID] = true
}

func (q *Queue) Pop() *DownloadRequest {
	res := make(chan *DownloadRequest)

	go func() {
		defer close(res)
		for {
			select {
			case req := <-q.hpQueue:
				res <- req
				return
			case req := <-q.lpQueue:
				res <- req
				return
			}
		}
	}()

	request := <-res

	q.lock.Lock()
	delete(q.activeIDs, request.chunkID)
	q.lock.Unlock()

	return request
}
