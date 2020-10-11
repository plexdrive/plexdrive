package chunk

import (
	"container/list"
	"sync"
)

// Stack is a thread safe list/stack implementation
type Stack struct {
	items   *list.List
	index   map[int]*list.Element
	lock    sync.Mutex
	maxSize int
}

// NewStack creates a new stack
func NewStack(maxChunks int) *Stack {
	return &Stack{
		items:   list.New(),
		index:   make(map[int]*list.Element, maxChunks),
		maxSize: maxChunks,
	}
}

// Len gets the number of items on the stack
func (s *Stack) Len() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.items.Len()
}

// Pop pops the first item from the stack
func (s *Stack) Pop() int {
	s.lock.Lock()
	if s.items.Len() < s.maxSize {
		s.lock.Unlock()
		return -1
	}

	item := s.items.Front()
	if nil == item {
		s.lock.Unlock()
		return -1
	}
	s.items.Remove(item)
	id := item.Value.(int)
	delete(s.index, id)
	s.lock.Unlock()

	return id
}

// Touch moves the specified item to the last position of the stack
func (s *Stack) Touch(id int) {
	s.lock.Lock()
	item, exists := s.index[id]
	if exists && item != s.items.Back() {
		s.items.MoveToBack(item)
	}
	s.lock.Unlock()
}

// Push adds a new item to the last position of the stack
func (s *Stack) Push(id int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, exists := s.index[id]; exists {
		return
	}
	s.index[id] = s.items.PushBack(id)
}

// Prepend adds a list to the front of the stack
func (s *Stack) Prepend(items *list.List) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for item := items.Front(); item != nil; item = item.Next() {
		id := item.Value.(int)
		s.index[id] = item
	}
	s.items.PushFrontList(items)
}

// Purge an item from the stack
func (s *Stack) Purge(id int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	item, exists := s.index[id]
	if exists && item != s.items.Front() {
		s.items.MoveToFront(item)
	}
}
