package chunk

import (
	"container/list"
	"sync"
)

// Stack is a thread safe list/stack implementation
type Stack struct {
	items   *list.List
	index   map[string]*list.Element
	len     int
	lock    sync.Mutex
	maxSize int
}

// NewStack creates a new stack
func NewStack(maxChunks int) *Stack {
	return &Stack{
		items:   list.New(),
		index:   make(map[string]*list.Element, maxChunks),
		maxSize: maxChunks,
	}
}

// Pop pops the first item from the stack
func (s *Stack) Pop() string {
	s.lock.Lock()
	if s.len < s.maxSize {
		s.lock.Unlock()
		return ""
	}

	item := s.items.Front()
	if nil == item {
		s.lock.Unlock()
		return ""
	}
	s.items.Remove(item)
	s.len--
	id := item.Value.(string)
	delete(s.index, id)
	s.lock.Unlock()

	return id
}

// Touch moves the specified item to the last position of the stack
func (s *Stack) Touch(id string) {
	s.lock.Lock()
	item, exists := s.index[id]
	if exists {
		s.items.MoveToBack(item)
	}
	s.lock.Unlock()
}

// Push adds a new item to the last position of the stack
func (s *Stack) Push(id string) {
	s.lock.Lock()
	if _, exists := s.index[id]; exists {
		s.lock.Unlock()
		return
	}
	s.items.PushBack(id)
	s.index[id] = s.items.Back()
	s.len++
	s.lock.Unlock()
}
