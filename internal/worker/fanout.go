package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

type FanoutSource string

const FanoutSourceKey FanoutSource = "fanoutSource"

type FanOut struct {
	SourceName  FanoutSource
	subscribers map[string]chan Task
	lock        sync.RWMutex
}

func NewFanOut(name string) *FanOut {
	return &FanOut{
		SourceName:  FanoutSource(name),
		subscribers: make(map[string]chan Task),
	}
}

func SetFanoutSource(f *FanOut, ctx context.Context) context.Context {
	return context.WithValue(ctx, FanoutSourceKey, f.SourceName)
}

func GetFanoutSource(ctx context.Context) string {
	val := ctx.Value(FanoutSourceKey).(FanoutSource)
	p := string(val)
	if p == "" {
		return "local"
	}
	return p
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
