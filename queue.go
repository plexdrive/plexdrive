package main

import (
	"container/list"
	"sync"
)

type Queue struct {
	items *list.List
	lock  sync.Mutex
}

type QueueItem struct {
	request  *ChunkRequest
	response chan *ChunkResponse
}

func NewQueue() *Queue {
	q := &Queue{
		items: list.New(),
	}
	return q
}

func (q *Queue) PushLeft(req *ChunkRequest) <-chan *ChunkResponse {
	res := make(chan *ChunkResponse)

	q.lock.Lock()
	q.items.PushFront(&QueueItem{
		request:  req,
		response: res,
	})
	q.lock.Unlock()

	return res
}

func (q *Queue) PushRight(req *ChunkRequest) <-chan *ChunkResponse {
	res := make(chan *ChunkResponse)

	q.lock.Lock()
	q.items.PushBack(&QueueItem{
		request:  req,
		response: res,
	})
	q.lock.Unlock()

	return res
}

func (q *Queue) Pop() (*ChunkRequest, chan *ChunkResponse) {
	res := make(chan *QueueItem)

	go func() {
		for {
			q.lock.Lock()
			item := q.items.Front()
			if nil != item {
				q.items.Remove(item)
				res <- item.Value.(*QueueItem)
				close(res)
				q.lock.Unlock()
				return
			}
			q.lock.Unlock()
		}
	}()

	result := <-res
	return result.request, result.response
}
