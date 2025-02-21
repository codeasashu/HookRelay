package worker

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/metrics"
)

var (
	ErrTooManyRetry = errors.New("job: too many retries")
	ErrRemoteJob    = errors.New("job: remote job error")
)

//	type Job struct {
//		ID            string
//		Event         *event.Event
//		Subscription  *subscription.Subscription
//		Result        *event.EventDelivery
//		numDeliveries uint16
//		wp            *WorkerPool
//		isRetrying    bool
//	}
//
//	type WorkerClient interface {
//		SendJob(job *delivery.EventDelivery) error
//	}
//
//	type Worker struct {
//		ID     string
//		client WorkerClient
//	}
//
//	func NewWorker() *Worker {
//		return &Worker{
//			ID: ulid.Make().String(),
//		}
//	}

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
}

type Task interface {
	GetID() string
	Execute() error
	Retries() int
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
	slog.Info("added local worker")
	return nil
}

func (wp *WorkerPool) SetQueueClient(c Worker) error {
	if err := c.Ping(); err != nil {
		slog.Warn("remote worker queue is not connected. only local workers will be used", "err", err)
		return err
	}
	wp.queueWorker = c
	slog.Info("queue worker client is connected")
	return nil
}

func (wp *WorkerPool) ShouldUseRemote(job Task, isRetrying bool) bool {
	// Checks if jobs should be scheduled to remote worker
	// based on several criterias
	remoteIsReady := wp.queueWorker != nil && wp.queueWorker.IsReady()
	localIsReady := wp.localWorker != nil && wp.localWorker.IsReady()
	if isRetrying {
		return remoteIsReady
	}
	return remoteIsReady && !localIsReady
}

func (wp *WorkerPool) Schedule(job Task, isRetrying bool) error {
	if wp.ShouldUseRemote(job, isRetrying) {
		slog.Info("scheduling job to queue worker", "job", job)
		return wp.queueWorker.Enqueue(job)
	}
	if wp.localWorker != nil {
		slog.Info("scheduling job to local worker", "job", job)
		return wp.localWorker.Enqueue(job)
	}
	return errors.New("error scheduling job. no worker available")
}

func (wp *WorkerPool) Shutdown() {
	if wp.localWorker != nil {
		wp.localWorker.Shutdown()
	}
	if wp.queueWorker != nil {
		wp.queueWorker.Shutdown()
	}
}
