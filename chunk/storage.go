package chunk

import (
	"sync"

	"time"

	. "github.com/claudetech/loggo/default"
)

type Storage struct {
	chunks map[string][]byte
	lock   sync.Mutex
	queue  chan *Item
}

type Item struct {
	id    string
	bytes []byte
}

func NewStorage() *Storage {
	storage := Storage{
		chunks: make(map[string][]byte),
		queue:  make(chan *Item, 100),
	}

	go storage.thread()

	return &storage
}

func (s *Storage) Store(id string, bytes []byte) error {
	s.lock.Lock()
	s.chunks[id] = bytes
	s.lock.Unlock()

	return nil
}

func (s *Storage) Get(id string) ([]byte, error) {
	res := make(chan []byte)

	go func() {
		for {
			bytes, exists := s.chunks[id]
			if exists {
				res <- bytes
				return
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()

	return <-res, nil
}

func (s *Storage) thread() {
	for {
		item := <-s.queue
		if err := s.storeToDisk(item.id, item.bytes); nil != err {
			Log.Warningf("%v", err)
		}
	}
}

func (s *Storage) storeToDisk(id string, bytes []byte) error {
	// TODO: implement
	return nil
}
