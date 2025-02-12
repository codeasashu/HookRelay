package wal

import (
	"fmt"
	"log/slog"
	sync "sync"
	"time"

	"github.com/codeasashu/HookRelay/internal/event"
)

type key string

const (
	WorkerContextKey key = "_wal_instance" // To get isntance by context key, in workers
)

type AbstractWAL interface {
	LogEvent(e *event.Event) error
	LogEventDelivery(e *event.EventDelivery) error
	LogBatchEventDelivery(events []*event.EventDelivery) error
	Close() error
	Init(t time.Time) error
	ForEachEvent(f func(e event.Event) error) error
}

var (
	ticker *time.Ticker
	stopCh chan struct{}
	mu     sync.Mutex
)

func init() {
	ticker = time.NewTicker(1 * time.Minute)
	stopCh = make(chan struct{})
}

func rotateWAL(wl AbstractWAL) {
	mu.Lock()
	defer mu.Unlock()

	if err := wl.Close(); err != nil {
		slog.Warn("error closing the WAL", slog.Any("error", err))
	}

	if err := wl.Init(time.Now()); err != nil {
		slog.Warn("error creating a new WAL", slog.Any("error", err))
	}
}

func periodicRotate(wl AbstractWAL) {
	for {
		select {
		case <-ticker.C:
			rotateWAL(wl)
		case <-stopCh:
			return
		}
	}
}

func InitBG(wl AbstractWAL) {
	go periodicRotate(wl)
}

func ShutdownBG() {
	close(stopCh)
	ticker.Stop()
}

func ReplayWAL(wl AbstractWAL) {
	err := wl.ForEachEvent(func(c event.Event) error {
		fmt.Println("replaying", c.UID, c.EventType)
		return nil
	})
	if err != nil {
		slog.Warn("error replaying WAL", slog.Any("error", err))
	}
}
