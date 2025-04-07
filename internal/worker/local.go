package worker

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/oklog/ulid/v2"
)

type LocalWorker struct {
	ID            string
	metrics       *metrics.Metrics
	queueuSize    int
	ctx           context.Context
	client        string
	JobQueue      chan Task
	MaxThreads    int            // Maximum allowed threads
	MinThreads    int            // Minimum threads to keep alive
	activeThreads int32          // Active thread count (atomic counter)
	StopChan      chan struct{}  // Signal for stopping the worker
	wg            sync.WaitGroup // WaitGroup for graceful shutdown
	Fanout        *FanOut

	seen sync.Map

	wp *WorkerPool
}

func NewLocalWorker(f *app.HookRelayApp, wp *WorkerPool) *LocalWorker {
	minThreads := f.Cfg.LocalWorker.MinThreads
	maxThreads := f.Cfg.LocalWorker.MaxThreads
	if maxThreads != -1 && maxThreads < minThreads {
		slog.Warn("max threads less than min thread. updating", "from", maxThreads, "to", minThreads)
		maxThreads = minThreads
	}

	localClient := &LocalWorker{
		ID:         "local-" + ulid.Make().String(),
		metrics:    f.Metrics,
		ctx:        f.Ctx,
		queueuSize: f.Cfg.LocalWorker.QueueSize,
		client:     "",
		JobQueue:   make(chan Task, f.Cfg.LocalWorker.QueueSize),
		MaxThreads: maxThreads,
		MinThreads: minThreads,
		StopChan:   make(chan struct{}),
		wp:         wp,
		Fanout:     NewFanOut("local"),
	}

	localClient.ReceiveJob()

	return localClient
}

func (c *LocalWorker) IsReady() bool {
	// Returns true if the queue is less than 40% full (coz only half the queue is alloted to JobQueue)
	return len(c.JobQueue) < int(math.Ceil((float64(c.queueuSize)/10)*8))
}

func (c *LocalWorker) Ping() error {
	// local worker is always ready
	return nil
}

func (c *LocalWorker) Enqueue(job Task) error {
	c.JobQueue <- job
	return nil
}

func (c *LocalWorker) ReceiveJob() {
	slog.Info("setting up local worker threads", "count", c.MinThreads)
	for range c.MinThreads {
		c.launchThread()
	}
	go c.scaleThreads(1 * time.Second)
}

func (c *LocalWorker) scaleThreads(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C: // Periodic check
			queueLen := len(c.JobQueue)
			active := atomic.LoadInt32(&c.activeThreads)
			c.metrics.UpdateWorkerQueueSize(queueLen)
			if queueLen > 0 && (c.MaxThreads == -1 || active < int32(c.MaxThreads)) {
				// Increase threads if the queue is filling up.
				slog.Debug("increasing worker threas", "worker_id", c.ID)
				c.launchThread()
			} else if queueLen == 0 && active > int32(c.MinThreads) {
				// Reduce threads if the queue is empty.
				slog.Debug("decreasing worker threads", "worker_id", c.ID)
				c.terminateThread()
			}
		case <-c.StopChan:
			return
		}
	}
}

func (w *LocalWorker) GetMetricsHandler() *metrics.Metrics {
	return w.metrics
}

func (w *LocalWorker) GetID() string {
	return w.ID
}

func (w *LocalWorker) GetType() WorkerType {
	return WorkerType("local")
}

func (w *LocalWorker) BroadcastResult(t Task) {
	if w.Fanout != nil {
		w.Fanout.Broadcast(t)
	}
}

// launchThread starts a new thread to process jobs.
func (w *LocalWorker) launchThread() {
	w.wg.Add(1)
	atomic.AddInt32(&w.activeThreads, 1)

	slog.Debug("Increased Worker threads by 1", "worker_id", w.ID, "current_count", w.activeThreads)
	go func() {
		defer func() {
			w.wg.Done()
			atomic.AddInt32(&w.activeThreads, -1)
			slog.Debug("Decreased Worker threads by 1", "worker_id", w.ID, "current_count", w.activeThreads)
		}()

		for {
			select {
			case task := <-w.JobQueue:
				slog.Info("local worker processing task", "trace_id", task.GetTraceID())
				err := task.Execute(w)
				task.IncDeliveries()
				retryErr := w.handleRetry(task, err)
				if retryErr != nil {
					slog.Error("retry error", "err", retryErr)
				}
				w.BroadcastResult(task)
			case <-w.StopChan:
				return
			}
		}
	}()
}

func (w *LocalWorker) handleRetry(t Task, err error) error {
	jobId := t.GetID()
	defaultVal := int32(0)
	count, _ := w.seen.LoadOrStore(jobId, &defaultVal)
	duplicateCount := *count.(*int32) // copy by value, for comparisons
	newCount := atomic.AddInt32(count.(*int32), 1)
	w.seen.Store(jobId, &newCount)

	if err != nil {
		if int(duplicateCount) < t.Retries() {
			slog.Info("retrying task", "trace_id", t.GetTraceID())
			w.wp.Schedule(t, true)
		} else {
			return ErrTooManyRetry
		}
	}

	return nil
}

// terminateThread stops a thread by signaling a reduction in workload.
func (w *LocalWorker) terminateThread() {
	// No specific mechanism to stop a thread; threads exit naturally when StopChan is closed.
	w.StopChan <- struct{}{}
}

// Stop gracefully stops the worker.
func (w *LocalWorker) Shutdown() {
	close(w.StopChan)
	w.wg.Wait()
	close(w.JobQueue)
}
