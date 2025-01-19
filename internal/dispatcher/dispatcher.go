package dispatcher

import (
	"log"
	"time"

	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/worker"

	"github.com/codeasashu/HookRelay/pkg/subscription"
)

type Dispatcher struct {
	Workers    []*worker.Worker
	JobResults []*worker.JobResult
	ErrJobs    []*worker.Job
	OkJobs     []*worker.Job
}

func NewDispatcher() *Dispatcher {
	wrk := worker.NewWorker(1, -1, -1)
	return &Dispatcher{
		Workers: []*worker.Worker{wrk},
	}
}

func (d *Dispatcher) AddWorker(worker *worker.Worker) {
	d.Workers = append(d.Workers, worker)
}

func (d *Dispatcher) Start() {
	// Start workers
	for _, wrk := range d.Workers {
		wrk.Start()
	}

	// Listen for results
	for _, wrk := range d.Workers {
		go d.listenResults(wrk)
	}
}

func (d *Dispatcher) listenResults(wrk *worker.Worker) {
	for jobResult := range wrk.ResultQueue {
		d.JobResults = append(d.JobResults, jobResult)
		log.Printf("Job %s completed with status=%s\n", jobResult.Job.ID, jobResult.Status)

		if jobResult.Status == "success" {
			d.OkJobs = append(d.OkJobs, jobResult.Job)
		} else {
			d.ErrJobs = append(d.ErrJobs, jobResult.Job)
		}
	}
}

func (d *Dispatcher) getAvailableWorker() *worker.Worker {
	return d.Workers[0]
}

func (d *Dispatcher) ListenForEvents(eventChannel <-chan event.Event) {
	for event := range eventChannel {
		log.Printf("Dispatching Event - %s\n", event.EventType)
		// event.AddLatencyTimestamp("dispatcher_start")
		subscriptions := subscription.GetSubscriptionsByEventType(event.EventType)
		log.Printf("Found %d subscriptions for event type - %s\n", len(subscriptions), event.EventType)
		if len(subscriptions) == 0 {
			// event.AddLatencyTimestamp("dispatcher_end")
			continue
		}
		// Fanout all the subscriptions for concurrent execution
		for _, sub := range subscriptions {
			job := &worker.Job{
				ID:           "job-" + event.EventType + "-" + time.Now().String(),
				Event:        &event,
				Subscription: sub,
			}
			wrk := d.getAvailableWorker()
			log.Printf("Scheduled job %s for event type - %s\n", job.ID, event.EventType)
			// event.AddLatencyTimestamp("dispatcher_end")
			wrk.JobQueue <- job
		}
	}
}

func (d *Dispatcher) Stop() {
	log.Println("Shutting down dispatcher...")
	for _, worker := range d.Workers {
		worker.Stop()
	}
	log.Printf("Total %d jobs processed, succeeded=%d, failed=%d\n", len(d.JobResults), len(d.OkJobs), len(d.ErrJobs))
}
