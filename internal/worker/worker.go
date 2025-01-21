package worker

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/pkg/subscription"
)

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
	ID            int
	JobQueue      chan *Job
	ResultQueue   chan *JobResult
	MaxThreads    int            // Maximum allowed threads
	MinThreads    int            // Minimum threads to keep alive
	activeThreads int32          // Active thread count (atomic counter)
	StopChan      chan struct{}  // Signal for stopping the worker
	wg            sync.WaitGroup // WaitGroup for graceful shutdown
}

func NewWorker(id, minThreads, maxThreads int) *Worker {
	if minThreads == -1 {
		minThreads = config.HRConfig.Worker.MinThreads
	}
	if maxThreads == -1 {
		maxThreads = config.HRConfig.Worker.MaxThreads
	}
	if maxThreads < minThreads {
		log.Printf("Max threads less than min thread. updated from %d to %d\n", maxThreads, minThreads)
		maxThreads = minThreads
	}

	if maxThreads > config.HRConfig.Worker.MaxThreads {
		log.Printf("Max threads exceeds hard limit. updated from %d to %d\n", maxThreads, config.HRConfig.Worker.MaxThreads)
		maxThreads = config.HRConfig.Worker.MaxThreads
	}
	return &Worker{
		ID:          id,                    // Worker ID
		JobQueue:    make(chan *Job, 1000), // Buffer size for job queue
		ResultQueue: make(chan *JobResult, 1000),
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

			if queueLen > 0 && active < int32(w.MaxThreads) {
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

	// log.Printf("Increased Worker %d threads from %d to %d\n", w.ID, active, active+1)
	go func() {
		defer func() {
			w.wg.Done()
			// active := atomic.LoadInt32(&w.activeThreads)
			atomic.AddInt32(&w.activeThreads, -1)
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
	// time.Sleep(5 * time.Second) // Simulate processing time.
	return job.Subscription.Target.ProcessTarget(job.Event.Payload)
	// return job.Subscription.Target.ProcessTarget(job.Event.Payload)
}
