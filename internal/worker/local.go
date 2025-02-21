package worker

import (
	"context"
	"log/slog"
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
	resultQueue   chan Task
	MaxThreads    int            // Maximum allowed threads
	MinThreads    int            // Minimum threads to keep alive
	activeThreads int32          // Active thread count (atomic counter)
	StopChan      chan struct{}  // Signal for stopping the worker
	wg            sync.WaitGroup // WaitGroup for graceful shutdown

	seen sync.Map

	wp *WorkerPool
}

func NewLocalWorker(f *app.HookRelayApp, wp *WorkerPool, callback func([]Task) error) *LocalWorker {
	minThreads := f.Cfg.LocalWorker.MinThreads
	maxThreads := f.Cfg.LocalWorker.MaxThreads
	if maxThreads != -1 && maxThreads < minThreads {
		slog.Warn("max threads less than min thread. updating", "from", maxThreads, "to", minThreads)
		maxThreads = minThreads
	}

	localClient := &LocalWorker{
		ID:          ulid.Make().String(),
		metrics:     f.Metrics,
		ctx:         f.Ctx,
		queueuSize:  f.Cfg.LocalWorker.QueueSize,
		client:      "",
		JobQueue:    make(chan Task, f.Cfg.LocalWorker.QueueSize/2), // Buffer size for sending events
		resultQueue: make(chan Task, f.Cfg.LocalWorker.QueueSize/2), // Buffer size for processing results
		MaxThreads:  maxThreads,
		MinThreads:  minThreads,
		StopChan:    make(chan struct{}),
		wp:          wp,
	}
	slog.Info("staring pool of local workers", "children", f.Cfg.LocalWorker.ResultHandlerThreads)
	for i := 0; i < f.Cfg.LocalWorker.ResultHandlerThreads; i++ {
		go ProcessBatchResults(localClient.resultQueue, callback)
	}

	localClient.ReceiveJob()

	return localClient
}

func (c *LocalWorker) CurrentCapacity() int {
	return len(c.JobQueue)
}

func (c *LocalWorker) IsNearlyFull() bool {
	// Returns true if the queue is more than 40% full (coz only half the queue is alloted to JobQueue)
	slog.Info("queue_size", "job_queue", len(c.JobQueue), "config", c.queueuSize)
	return len(c.JobQueue) > (c.queueuSize/10)*4
}

func (c *LocalWorker) IsReady() bool {
	// Returns true if the queue is less than 40% full (coz only half the queue is alloted to JobQueue)
	return len(c.JobQueue) > (c.queueuSize/10)*4
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
	for i := 0; i < c.MinThreads; i++ {
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
			c.metrics.UpdateWorkerQueueSize("local", queueLen)
			c.metrics.UpdateWorkerThreadCount("local", int(active))

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

// launchThread starts a new thread to process jobs.
func (w *LocalWorker) launchThread() {
	// app := cli.GetAppInstance()
	w.wg.Add(1)
	atomic.AddInt32(&w.activeThreads, 1)
	w.metrics.UpdateWorkerThreadCount("local", int(w.activeThreads))

	slog.Debug("Increased Worker threads by 1", "worker_id", w.ID, "current_count", w.activeThreads)
	go func() {
		defer func() {
			w.wg.Done()
			atomic.AddInt32(&w.activeThreads, -1)
			w.metrics.UpdateWorkerThreadCount("local", int(w.activeThreads))
			slog.Debug("Decreased Worker threads by 1", "worker_id", w.ID, "current_count", w.activeThreads)
		}()

		for {
			select {
			case job := <-w.JobQueue:
				slog.Info("got job item", "job_id", job.GetID())
				// w.metrics.RecordDispatchLatency(job., "local") // @TODO: Fix me
				err := job.Execute()
				retryErr := w.handleRetry(job, err)
				if retryErr != nil {
					slog.Error("retry error", "err", retryErr)
				}
				w.resultQueue <- job
			case <-w.StopChan:
				return
			}
		}
	}()
}

func (w *LocalWorker) handleRetry(job Task, err error) error {
	jobId := job.GetID()
	defaultVal := int32(0)
	count, _ := w.seen.LoadOrStore(jobId, &defaultVal)
	duplicateCount := *count.(*int32) // copy by value, for comparisons
	newCount := atomic.AddInt32(count.(*int32), 1)
	w.seen.Store(jobId, &newCount)

	if err != nil {
		if int(duplicateCount) < job.Retries() {
			w.wp.Schedule(job, true)
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
	close(w.resultQueue)
}

func ProcessBatchResults(jobChan <-chan Task, callback func([]Task) error) {
	const batchSize = 100
	const flushInterval = 500 * time.Millisecond

	var batch []Task
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case job, ok := <-jobChan:
			if !ok {
				// Channel closed, flush remaining events
				if len(batch) > 0 {
					callback(batch)
				}
				return
			}

			// m.IncrementIngestConsumedTotal(job.Event, "local")
			batch = append(batch, job)

			if len(batch) >= batchSize {
				callback(batch)
				batch = nil // Reset batch
			}
		case <-ticker.C:
			if len(batch) > 0 {
				callback(batch)
				batch = nil // Reset batch
			}
		}
	}
}
