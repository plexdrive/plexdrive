package chunk

import (
	"container/list"
	"sync"
)

// Stack is a thread safe list/stack implementation
type Stack struct {
	items   *list.List
	lock    sync.Mutex
	maxSize int
}

// NewStack creates a new stack
func NewStack(maxChunks int) *Stack {
	return &Stack{
		items:   list.New(),
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
	defer s.lock.Unlock()
	if s.items.Len() < s.maxSize {
		return -1
	}
	item := s.items.Front()
	if nil == item {
		return -1
	}
	s.items.Remove(item)
	return item.Value.(int)
}

// Touch moves the specified item to the last position of the stack
func (s *Stack) Touch(item *list.Element) {
	s.lock.Lock()
	if item != s.items.Back() {
		s.items.MoveToBack(item)
	}
	s.lock.Unlock()
}

// Push adds a new item to the last position of the stack
func (s *Stack) Push(id int) *list.Element {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.items.PushBack(id)
}

// Prepend adds a list to the front of the stack
func (s *Stack) Prepend(items *list.List) {
	s.lock.Lock()
	s.items.PushFrontList(items)
	s.lock.Unlock()
}

// Purge an item from the stack
func (s *Stack) Purge(item *list.Element) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if item != s.items.Front() {
		s.items.MoveToFront(item)
	}
}
