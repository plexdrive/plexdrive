package chunk

import (
	"container/list"
	"sync"
)

type Queue struct {
	items  *list.List
	lock   sync.Mutex
	listen chan int
}

type QueueItem struct {
	request  *ChunkRequest
	response chan *ChunkResponse
}

func NewQueue() *Queue {
	q := &Queue{
		items:  list.New(),
		listen: make(chan int),
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
	q.listen <- 1
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
	q.listen <- 1
	q.lock.Unlock()

	return res
}

func (q *Queue) Pop() (*ChunkRequest, chan *ChunkResponse) {
	if q.items.Len() <= 0 {
		<-q.listen
	}

	q.lock.Lock()
	item := q.items.Front()
	if nil == item {
		q.lock.Unlock()
		return q.Pop()
	}

	q.items.Remove(item)
	q.lock.Unlock()

	result := item.Value.(*QueueItem)
	return result.request, result.response
}
