package worker

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/pkg/subscription"

	"github.com/oklog/ulid/v2"
)

var m *metrics.Metrics

type Job struct {
	ID           string
	Event        *event.Event
	Subscription *subscription.Subscription
}

type JobResult struct {
	Job    *Job
	Status string
	Error  error
}

type WorkerPool struct {
	workers []*Worker
	mutex   sync.Mutex
}

type WorkerClient interface {
	SendJob(job *Job) error
	ReceiveJob(chan<- *Job)
}

type Worker struct {
	ID            string
	JobQueue      chan *Job
	ResultQueue   chan *JobResult
	MaxThreads    int            // Maximum allowed threads
	MinThreads    int            // Minimum threads to keep alive
	activeThreads int32          // Active thread count (atomic counter)
	StopChan      chan struct{}  // Signal for stopping the worker
	wg            sync.WaitGroup // WaitGroup for graceful shutdown
	client        WorkerClient
}

func NewWorker() *Worker {
	minThreads := config.HRConfig.Worker.MinThreads
	maxThreads := config.HRConfig.Worker.MaxThreads
	if maxThreads != -1 && maxThreads < minThreads {
		slog.Warn("max threads less than min thread. updating", "from", maxThreads, "to", minThreads)
		maxThreads = minThreads
	}

	m = metrics.GetDPInstance()
	return &Worker{
		ID:          ulid.Make().String(),
		JobQueue:    make(chan *Job, config.HRConfig.Worker.QueueSize/2), // Buffer size for job queue
		ResultQueue: make(chan *JobResult, config.HRConfig.Worker.QueueSize/2),
		MaxThreads:  maxThreads,
		MinThreads:  minThreads,
		StopChan:    make(chan struct{}),
	}
}

func (w *Worker) ReceiveJob(onChan chan *Job) {
	if w.client != nil {
		w.client.ReceiveJob(onChan)
	}
}

func (w *Worker) DispatchJob(job *Job) error {
	if w.client != nil {
		w.client.SendJob(job)
	}

	// if w.pubsubClient != nil {
	// 	t, err := NewQueueJob(job)
	// 	if err != nil {
	// 		slog.Error("error creating redis task", "err", err)
	// 		return err
	// 	}
	// 	info, err := w.queueClient.Enqueue(t, asynq.Queue("hookrelay"))
	// 	if err != nil {
	// 		slog.Error("could not enqueue task", "err", err)
	// 		return err
	// 	}
	// 	slog.Info("enqueued task", "task_id", info.ID, "queue", info.Queue)
	// 	return nil
	// }
	return nil
}

func (w *Worker) Start() {
	slog.Info("launching worker threads", "worker", w.ID, "threads", w.MinThreads, "max_threads", w.MaxThreads)
	// Start with the minimum number of threads.
	for i := 0; i < w.MinThreads; i++ {
		w.launchThread()
	}

	// Start a goroutine to monitor and scale threads dynamically.
	go w.scaleThreads(1 * time.Second)
}

func (w *Worker) scaleThreads(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C: // Periodic check
			queueLen := len(w.JobQueue)
			active := atomic.LoadInt32(&w.activeThreads)
			m.UpdateWorkerQueueSize(w.ID, queueLen)
			m.UpdateWorkerThreadCount(w.ID, int(active))

			if queueLen > 0 && (w.MaxThreads == -1 || active < int32(w.MaxThreads)) {
				// Increase threads if the queue is filling up.
				slog.Debug("increasing worker threas", "worker_id", w.ID)
				w.launchThread()
			} else if queueLen == 0 && active > int32(w.MinThreads) {
				// Reduce threads if the queue is empty.
				slog.Debug("decreasing worker threads", "worker_id", w.ID)
				w.terminateThread()
			}
		case <-w.StopChan:
			return
		}
	}
}

// launchThread starts a new thread to process jobs.
func (w *Worker) launchThread() {
	w.wg.Add(1)
	// active := atomic.LoadInt32(&w.activeThreads)
	atomic.AddInt32(&w.activeThreads, 1)
	m.UpdateWorkerThreadCount(w.ID, int(w.activeThreads))

	// slog.Info("Increased Worker %d threads from %d to %d\n", w.ID, active, active+1)
	go func() {
		defer func() {
			w.wg.Done()
			// active := atomic.LoadInt32(&w.activeThreads)
			atomic.AddInt32(&w.activeThreads, -1)
			m.UpdateWorkerThreadCount(w.ID, int(w.activeThreads))
			// slog.Info("Decreased Worker %d threads from %d to %d\n", w.ID, active, active-1)
		}()

		for {
			select {
			case job := <-w.JobQueue:
				job.Subscription.Dispatch()
				// Simulate job processing.
				err := processJob(job)
				job.Subscription.Complete()
				status := "success"
				if err != nil {
					status = "failed"
				}

				// Send result to result queue.
				slog.Info("job complete. sending result", "job_id", job.ID)
				w.ResultQueue <- &JobResult{
					Job:    job,
					Status: status,
					Error:  err,
				}
			case <-w.StopChan:
				return
			}
		}
	}()
}

// terminateThread stops a thread by signaling a reduction in workload.
func (w *Worker) terminateThread() {
	// No specific mechanism to stop a thread; threads exit naturally when StopChan is closed.
	w.StopChan <- struct{}{}
}

// Stop gracefully stops the worker.
func (w *Worker) Stop() {
	close(w.StopChan)
	w.wg.Wait()
	close(w.JobQueue)
	close(w.ResultQueue)
}

// processJob simulates the actual job processing (e.g., an HTTP call).
func processJob(job *Job) error {
	// time.Sleep(3 * time.Second) // Simulate processing time.
	// return nil
	return job.Subscription.Target.ProcessTarget(job.Event.Payload)
}
