package wal

import (
	"errors"
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
	ForEachEventDeliveriesBatch(batchSize int, f func(e []*event.EventDelivery) error) error
	Shutdown() error
}

var (
	rotateTicker *time.Ticker
	replayTicker *time.Ticker
	stopCh       chan struct{}
	mu           sync.Mutex
)

func init() {
	rotateTicker = time.NewTicker(1 * time.Minute)
	replayTicker = time.NewTicker(5 * time.Second)
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
	slog.Info("starting periodic rotate worker")
	for {
		select {
		case <-rotateTicker.C:
			rotateWAL(wl)
		case <-stopCh:
			wl.Shutdown()
			return
		}
	}
}

func InitBG(wl AbstractWAL, batchSize int, callbacks []func([]*event.EventDelivery)) {
	go periodicRotate(wl)
	go periodicReplay(wl, batchSize, callbacks)
}

func ShutdownBG() {
	slog.Info("shutting down WAL")
	close(stopCh)
	rotateTicker.Stop()
	replayTicker.Stop()
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

func periodicReplay(wl AbstractWAL, batchSize int, callbacks []func([]*event.EventDelivery)) {
	slog.Info("starting periodic replay worker", "batch", batchSize)
	for {
		select {
		case <-replayTicker.C:
			replayWALBatch(wl, batchSize, callbacks)
		case <-stopCh:
			return
		}
	}
}

func replayWALBatch(wl AbstractWAL, batchSize int, callbacks []func([]*event.EventDelivery)) {
	// var wg sync.WaitGroup
	// wg.Add(1) // @TODO: make it 2: 1=accounting, 1=recovery

	slog.Info("replaying event deliveries", "batch", batchSize)
	err := wl.ForEachEventDeliveriesBatch(batchSize, func(c []*event.EventDelivery) error {
		slog.Info("replayed event deliveries", "batch", len(c))
		for _, cb := range callbacks {
			// execute callbacks asyncronously
			go cb(c)
		}
		return nil
	})
	if err != nil {
		slog.Warn("error replaying WAL", slog.Any("error", err))
	}
	// wg.Wait()
	slog.Info("Sync and recovery completed")
}

func DoAccounting(accounting *Accounting, e []*event.EventDelivery) error {
	if accounting == nil {
		slog.Warn("accounting not initialized, skipping...")
		return errors.New("accounting not initialized")
	}
	accounting.CreateDeliveries(e)
	return nil
}

// if w.accounting != nil {
// 	w.accounting.db.Close()
// }
