package worker

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/metrics"
)

var (
	ErrTooManyRetry = errors.New("job: too many retries")
	ErrRemoteJob    = errors.New("job: remote job error")
)

type WorkerType string

const (
	LocalWorkerType WorkerType = "local"
	QueueWorkerType WorkerType = "queue"
)

// @TODO: Make this a linked list to support multiple workers
type WorkerPool struct {
	mu          *sync.Mutex
	metrics     *metrics.Metrics
	localWorker Worker
	queueWorker Worker
}

type Worker interface {
	Enqueue(t Task) error
	Ping() error
	// IsNearlyFull() bool
	Shutdown()
	IsReady() bool
	GetID() string
	GetType() WorkerType
	GetMetricsHandler() *metrics.Metrics
}

type Task interface {
	GetID() string
	GetTraceID() string
	GetType() string
	Execute(worker Worker) error
	Retries() int
	NumDeliveries() int // Get number of deliveries till now
	IncDeliveries()     // Increment deliveries
}

func InitPool(f *app.HookRelayApp) *WorkerPool {
	wp := &WorkerPool{
		mu:      &sync.Mutex{},
		metrics: f.Metrics,
	}
	return wp
}

func CreateLocalWorker(f *app.HookRelayApp, wp *WorkerPool, callback func(tasks []Task) error) *LocalWorker {
	lw := NewLocalWorker(f, wp, callback)
	return lw
}

func CreateQueueWorker(f *app.HookRelayApp, marshaler MarshalerMap, callback func([]Task) error) *QueueWorker {
	lw := NewQueueWorker(f, marshaler)
	return lw
}

func (wp *WorkerPool) SetLocalClient(c Worker) error {
	wp.localWorker = c
	slog.Info("local worker: connected")
	return nil
}

func (wp *WorkerPool) SetQueueClient(c Worker) error {
	if err := c.Ping(); err != nil {
		slog.Warn("remote worker queue is not connected. only local workers will be used", "err", err)
		return err
	}
	wp.queueWorker = c
	slog.Info("queue worker: connected")
	return nil
}

// Checks if jobs should be scheduled to remote worker
func (wp *WorkerPool) ShouldUseRemote(job Task, isRetrying bool) bool {
	remoteIsReady := wp.queueWorker != nil && wp.queueWorker.IsReady()
	localIsReady := wp.localWorker != nil && wp.localWorker.IsReady()

	// prioritise local worker
	return (!localIsReady && remoteIsReady) || (isRetrying && remoteIsReady)
}

func (wp *WorkerPool) Schedule(t Task, isRetrying bool) error {
	if wp.ShouldUseRemote(t, isRetrying) {
		slog.Info("scheduling job to queue worker", "trace_id", t.GetTraceID(), "isRetrying", isRetrying)
		return wp.queueWorker.Enqueue(t)
	}
	if wp.localWorker != nil {
		slog.Info("scheduling job to local worker", "trace_id", t.GetTraceID(), "isRetrying", isRetrying)
		return wp.localWorker.Enqueue(t)
	}
	return fmt.Errorf("error scheduling job (trace_id=%s). no worker available", t.GetTraceID())
}

func (wp *WorkerPool) Shutdown() {
	if wp.localWorker != nil {
		wp.localWorker.Shutdown()
	}
	if wp.queueWorker != nil {
		wp.queueWorker.Shutdown()
	}
}
