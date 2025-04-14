package wal

import (
	"database/sql"
	"log/slog"
	sync "sync"
	"time"
)

type key string

const (
	WorkerContextKey key = "_wal_instance" // To get isntance by context key, in workers
)

type AbstractWAL interface {
	Init(t time.Time) error
	GetDB() *sql.DB
	Log(callback func(db *sql.DB) error) error
	GetMutex() *sync.Mutex
	Close() error
	ForEachRotation(callbacks []func(db *sql.DB) error) error
}

var (
	rotateTicker *time.Ticker
	replayTicker *time.Ticker
	stopRotateCh chan struct{}
	stopReplayCh chan struct{}
	mu           sync.Mutex
)

func init() {
	rotateTicker = time.NewTicker(1 * time.Minute)
	replayTicker = time.NewTicker(5 * time.Second)
	stopRotateCh = make(chan struct{})
	stopReplayCh = make(chan struct{})
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
		case <-stopRotateCh:
			wl.Close()
			return
		}
	}
}

func InitBG(wl AbstractWAL, callbacks []func(db *sql.DB) error) {
	go periodicRotate(wl)
	go periodicReplay(wl, callbacks)
}

func ShutdownBG() {
	slog.Info("shutting down WAL")
	rotateTicker.Stop()
	replayTicker.Stop()
	close(stopRotateCh)
	close(stopReplayCh)
	time.Sleep(1 * time.Second)
}

func periodicReplay(wl AbstractWAL, callbacks []func(db *sql.DB) error) {
	slog.Info("starting periodic replay worker")
	for {
		select {
		case <-replayTicker.C:
			replayWALBatch(wl, callbacks)
		case <-stopReplayCh:
			wl.Close()
			return
		}
	}
}

func replayWALBatch(wl AbstractWAL, callbacks []func(db *sql.DB) error) {
	slog.Info("replaying event deliveries")
	err := wl.ForEachRotation(callbacks)
	if err != nil {
		slog.Warn("error replaying WAL", slog.Any("error", err))
	}
	slog.Info("Sync and recovery completed")
}

//
// func DoAccounting(accounting *Accounting, e []*delivery.EventDelivery) error {
// 	if accounting == nil {
// 		slog.Warn("accounting not initialized, skipping...")
// 		return errors.New("accounting not initialized")
// 	}
// 	accounting.CreateDeliveries(e)
// 	return nil
// }
//
