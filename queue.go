package main

import (
	"fmt"
	"sync"
)

type Queue struct {
	lock     *sync.Mutex
	elements []*QueueElement
}

type QueueElement struct {
	id   string
	item interface{}
}

func NewQueue() *Queue {
	return &Queue{
		lock:     &sync.Mutex{},
		elements: make([]*QueueElement, 100),
	}
}

func (q *Queue) Contains(id string) bool {
	q.lock.Lock()
	for _, element := range q.elements {
		if element.id == id {
			q.lock.Unlock()
			return true
		}
	}
	q.lock.Unlock()
	return false
}

func (q *Queue) PushHighPrio(id string, element interface{}) {
	if !q.Contains(id) {
		q.lock.Lock()
		q.elements = append([]*QueueElement{&QueueElement{
			id:   id,
			item: element,
		}}, q.elements...)
		q.lock.Unlock()
	}
}

func (q *Queue) PushLowPrio(id string, element interface{}) {
	if !q.Contains(id) {
		q.lock.Lock()
		q.elements = append(q.elements, &QueueElement{
			id:   id,
			item: element,
		})
		q.lock.Unlock()
	}
}

func (q *Queue) Pop() (interface{}, error) {
	var element *QueueElement

	q.lock.Lock()
	if len(q.elements) > 0 {
		element = q.elements[0]
		q.elements = q.elements[1:]
	}
	q.lock.Unlock()

	if nil == element {
		return nil, fmt.Errorf("not found")
	}
	return element.item, nil
}
