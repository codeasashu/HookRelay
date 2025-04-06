package worker

import (
	"errors"
	"log/slog"
	"sync"
)

type FanOut struct {
	subscribers map[string]chan Task
	lock        sync.RWMutex
}

func NewFanOut() *FanOut {
	return &FanOut{
		subscribers: make(map[string]chan Task),
	}
}

func (f *FanOut) Subscribe(name string, buffer int) (<-chan Task, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if _, found := f.subscribers[name]; found {
		return nil, errors.New("fanout: subscriber already exists")
	}
	c := make(chan Task, buffer)
	f.subscribers[name] = c
	return c, nil
}

func (f *FanOut) Unsubscribe(name string) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if ch, ok := f.subscribers[name]; ok {
		close(ch)
		delete(f.subscribers, name)
	}
}

func (f *FanOut) Broadcast(task Task) {
	f.lock.RLock()
	defer f.lock.RUnlock()

	for name, ch := range f.subscribers {
		select {
		case ch <- task:
			// successfully sent
		default:
			slog.Error("fanout: subscriber queue full, dropping task", "subscriber", name)
		}
	}
}
