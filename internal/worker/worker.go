package worker

import (
	"log"
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

type Worker struct {
	ID            string
	JobQueue      chan *Job
	ResultQueue   chan *JobResult
	MaxThreads    int            // Maximum allowed threads
	MinThreads    int            // Minimum threads to keep alive
	activeThreads int32          // Active thread count (atomic counter)
	StopChan      chan struct{}  // Signal for stopping the worker
	wg            sync.WaitGroup // WaitGroup for graceful shutdown
}

func NewWorker() *Worker {
	minThreads := config.HRConfig.Worker.MinThreads
	maxThreads := config.HRConfig.Worker.MaxThreads
	if maxThreads != -1 && maxThreads < minThreads {
		log.Printf("Max threads less than min thread. updated from %d to %d\n", maxThreads, minThreads)
		maxThreads = minThreads
	}

	m = metrics.GetDPInstance()
	return &Worker{
		ID:          ulid.Make().String(),
		JobQueue:    make(chan *Job, 100-000), // Buffer size for job queue
		ResultQueue: make(chan *JobResult, 100-000),
		MaxThreads:  maxThreads,
		MinThreads:  minThreads,
		StopChan:    make(chan struct{}),
	}
}

func (w *Worker) Start() {
	log.Printf("Starting dispatcher worker %d with %d threads (max=%d) \n", w.ID, w.MinThreads, w.MaxThreads)
	// Start with the minimum number of threads.
	for i := 0; i < w.MinThreads; i++ {
		w.launchThread()
	}

	// Start a goroutine to monitor and scale threads dynamically.
	go w.scaleThreads(1 * time.Second)

	// // Start a result processing thread.
	// go func() {
	// 	for result := range w.ResultQueue {
	// 		// Handle result (e.g., announce to dispatcher).
	// 		log.Printf("Worker %d: Processed Job %s, Status: %s\n", w.ID, result.JobID, result.Status)
	// 	}
	// }()
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
				log.Printf("Increasing Worker %d threads\n", w.ID)
				w.launchThread()
			} else if queueLen == 0 && active > int32(w.MinThreads) {
				// Reduce threads if the queue is empty.
				log.Printf("Decreasing Worker %d threads\n", w.ID)
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

	// log.Printf("Increased Worker %d threads from %d to %d\n", w.ID, active, active+1)
	go func() {
		defer func() {
			w.wg.Done()
			// active := atomic.LoadInt32(&w.activeThreads)
			atomic.AddInt32(&w.activeThreads, -1)
			m.UpdateWorkerThreadCount(w.ID, int(w.activeThreads))
			// log.Printf("Decreased Worker %d threads from %d to %d\n", w.ID, active, active-1)
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
				log.Printf("Sending result to jobresult queue %s\n", job.ID)
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
	log.Printf("Processing job %s, Subscription=%s \n", job.ID, job.Subscription.Target.HTTPDetails.URL)
	// time.Sleep(3 * time.Second) // Simulate processing time.
	// return nil
	return job.Subscription.Target.ProcessTarget(job.Event.Payload)
}
