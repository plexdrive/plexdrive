package chunk

import (
	"container/list"
	"sync"
)

// Stack is a thread safe list/stack implementation
type Stack struct {
	items *list.List
	lock  sync.RWMutex
}

// NewStack creates a new stack
func NewStack() *Stack {
	return &Stack{
		items: list.New(),
	}
}

// Len returns the length of the current stack
func (s *Stack) Len() int {
	s.lock.RLock()
	count := s.items.Len()
	s.lock.RUnlock()
	return count
}

// Pop pops the first item from the stack
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

// Touch moves the specified item to the last position of the stack
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

// Push adds a new item to the last position of the stack
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
