package worker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/wal"
	"github.com/oklog/ulid/v2"
)

type LocalClient struct {
	ID            string
	app           *cli.App
	ctx           context.Context
	client        string
	JobQueue      chan *Job
	resultQueue   chan *Job
	MaxThreads    int            // Maximum allowed threads
	MinThreads    int            // Minimum threads to keep alive
	activeThreads int32          // Active thread count (atomic counter)
	StopChan      chan struct{}  // Signal for stopping the worker
	wg            sync.WaitGroup // WaitGroup for graceful shutdown
}

func NewLocalWorker(app *cli.App, wl wal.AbstractWAL) *Worker {
	minThreads := config.HRConfig.LocalWorker.MinThreads
	maxThreads := config.HRConfig.LocalWorker.MaxThreads
	if maxThreads != -1 && maxThreads < minThreads {
		slog.Warn("max threads less than min thread. updating", "from", maxThreads, "to", minThreads)
		maxThreads = minThreads
	}

	localClient := &LocalClient{
		ctx:         context.Background(),
		app:         app,
		client:      "",
		JobQueue:    make(chan *Job, config.HRConfig.LocalWorker.QueueSize/2), // Buffer size for sending events
		resultQueue: make(chan *Job, config.HRConfig.LocalWorker.QueueSize/2), // Buffer size for processing results
		MaxThreads:  maxThreads,
		MinThreads:  minThreads,
		StopChan:    make(chan struct{}),
	}
	m = metrics.GetDPInstance()
	w := &Worker{
		ID:     ulid.Make().String(),
		client: localClient,
	}

	slog.Info("staring pool of local workers", "children", config.HRConfig.LocalWorker.ResultHandlerThreads)
	for i := 0; i < config.HRConfig.LocalWorker.ResultHandlerThreads; i++ {
		go ProcessResultsFromLocalChan(localClient.resultQueue, wl)
	}

	localClient.ReceiveJob()

	return w
}

func (c *LocalClient) CurrentCapacity() int {
	return len(c.JobQueue)
}

func (c *LocalClient) IsNearlyFull() bool {
	// Returns true if the queue is more than 40% full (coz only half the queue is alloted to JobQueue)
	return len(c.JobQueue) > (config.HRConfig.LocalWorker.QueueSize/10)*4
}

func (c *LocalClient) SendJob(job *Job) error {
	c.JobQueue <- job
	return nil
}

func (c *LocalClient) ReceiveJob() {
	slog.Info("setting up local worker threads", "count", c.MinThreads)
	for i := 0; i < c.MinThreads; i++ {
		c.launchThread()
	}
	go c.scaleThreads(1 * time.Second)
}

func (c *LocalClient) scaleThreads(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C: // Periodic check
			queueLen := len(c.JobQueue)
			active := atomic.LoadInt32(&c.activeThreads)
			m.UpdateWorkerQueueSize("local", queueLen)
			m.UpdateWorkerThreadCount("local", int(active))

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
func (w *LocalClient) launchThread() {
	// app := cli.GetAppInstance()
	w.wg.Add(1)
	atomic.AddInt32(&w.activeThreads, 1)
	m.UpdateWorkerThreadCount("local", int(w.activeThreads))

	slog.Debug("Increased Worker threads by 1", "worker_id", w.ID, "current_count", w.activeThreads)
	go func() {
		defer func() {
			w.wg.Done()
			atomic.AddInt32(&w.activeThreads, -1)
			m.UpdateWorkerThreadCount("local", int(w.activeThreads))
			slog.Debug("Decreased Worker threads by 1", "worker_id", w.ID, "current_count", w.activeThreads)
		}()

		for {
			select {
			case job := <-w.JobQueue:
				slog.Info("got job item", "job_id", job.ID)
				m.RecordDispatchLatency(job.Event, "local")
				_, err := job.Exec() // Update job result
				if err != nil {
					retryErr := job.Retry()
					if retryErr == ErrTooManyRetry {
						slog.Error("job failed. Retry exhausted", "job_id", job.ID, "error", err)
					} else {
						slog.Error("error processing job", "job_id", job.ID, "error", err)
					}
				} else {
					slog.Info("job complete. sending result", "job_id", job.ID)
				}
				w.resultQueue <- job
			case <-w.StopChan:
				return
			}
		}
	}()
}

// terminateThread stops a thread by signaling a reduction in workload.
func (w *LocalClient) terminateThread() {
	// No specific mechanism to stop a thread; threads exit naturally when StopChan is closed.
	w.StopChan <- struct{}{}
}

// Stop gracefully stops the worker.
func (w *LocalClient) Stop() {
	close(w.StopChan)
	w.wg.Wait()
	close(w.JobQueue)
	close(w.resultQueue)
}

func ProcessResultsFromLocalChan(jobChan <-chan *Job, wl wal.AbstractWAL) {
	// @TODO: Dont insert delivery result right away, process in batch (maybe insert into redis)
	// deliveryModel := event.NewEventModel(app.DB)
	// deliveryModel.CreateEventDelivery(job.Result)
	for job := range jobChan {
		m.IncrementIngestConsumedTotal(job.Event, "local")
		// @TODO: Dont insert delivery result right away, process in batch (maybe insert into redis)
		wl.LogEventDelivery(job.Result)
		m.RecordEndToEndLatency(job.Result, "local")
	}
}
