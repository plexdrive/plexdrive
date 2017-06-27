package chunk

import (
	"container/list"
	"sync"
)

type Stack struct {
	items *list.List
	lock  sync.Mutex
}

func NewStack() *Stack {
	return &Stack{
		items: list.New(),
	}
}

func (s *Stack) Len() int {
	s.lock.Lock()
	count := s.items.Len()
	s.lock.Unlock()
	return count
}

func (s *Stack) Pop() string {
	s.lock.Lock()
	item := s.items.Front()
	if nil == item {
		s.lock.Unlock()
		return ""
	}
	s.items.Remove(item)
	s.lock.Unlock()

	return item.Value.(string)
}

func (s *Stack) Touch(id string) {
	s.lock.Lock()
	for item := s.items.Front(); item != nil; item = item.Next() {
		if item.Value.(string) == id {
			s.items.MoveToBack(item)
			break
		}
	}
	s.lock.Unlock()
}

func (s *Stack) Push(id string) {
	s.lock.Lock()
	for item := s.items.Front(); item != nil; item = item.Next() {
		if item.Value.(string) == id {
			s.lock.Unlock()
			return
		}
	}
	s.items.PushBack(id)
	s.lock.Unlock()
}
