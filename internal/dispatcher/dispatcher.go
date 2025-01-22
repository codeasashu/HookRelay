package dispatcher

import (
	"log/slog"
	"sync"
	"time"

	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/worker"

	"github.com/codeasashu/HookRelay/pkg/subscription"
)

var m *metrics.Metrics

type Dispatcher struct {
	lock       *sync.RWMutex
	Workers    []*worker.Worker
	JobResults map[string][]*worker.JobResult
	eventJobs  map[string]int
	totalJobs  int
}

func NewDispatcher() *Dispatcher {
	m = metrics.GetDPInstance()
	wrk := worker.NewWorker()
	return &Dispatcher{
		lock:       &sync.RWMutex{},
		Workers:    []*worker.Worker{wrk},
		JobResults: make(map[string][]*worker.JobResult),
		eventJobs:  make(map[string]int),
		totalJobs:  0,
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
		m.IncrementIngestConsumedTotal(jobResult.Job.Event)
		if jobResult.Status == "success" {
			m.IncrementIngestSuccessTotal(jobResult.Job.Event)
		} else {
			m.IncrementIngestErrorsTotal(jobResult.Job.Event)
		}
		d.lock.Lock()
		d.totalJobs++
		slog.Info("job completed", "job_id", jobResult.Job.ID, "status", jobResult.Status)
		if found := d.JobResults[jobResult.Job.Event.UID]; found != nil {
			d.JobResults[jobResult.Job.Event.UID] = append(d.JobResults[jobResult.Job.Event.UID], jobResult)
		} else {
			d.JobResults[jobResult.Job.Event.UID] = []*worker.JobResult{jobResult}
		}
		expectedJobs := d.eventJobs[jobResult.Job.Event.UID]
		actualJobs := len(d.JobResults[jobResult.Job.Event.UID])
		if expectedJobs > 0 && (expectedJobs == actualJobs) {
			m.RecordEndToEndLatency(jobResult.Job.Event)
			slog.Info("all jobs completed", "event_id", jobResult.Job.Event.UID, "completed_jobs", actualJobs)
		}
		d.lock.Unlock()
	}
}

func (d *Dispatcher) getAvailableWorker() *worker.Worker {
	return d.Workers[0]
}

func (d *Dispatcher) GetJobsByEventUID(eventUID string) []*worker.JobResult {
	return d.JobResults[eventUID]
}

func (d *Dispatcher) ListenForEvents(eventChannel <-chan event.Event) {
	for event := range eventChannel {
		slog.Info("dispatching event", "id", event.UID, "type", event.EventType)
		m.RecordPreFlightLatency(&event)
		subscriptions := subscription.GetSubscriptionsByEventType(event.EventType)
		slog.Info("fetched subscriptions", "event_id", event.UID, "fanout", len(subscriptions), "event_type", event.EventType)
		m.RecordFanout(&event, len(subscriptions))
		if len(subscriptions) == 0 {
			event.CompletedAt = time.Now()
			continue
		}
		d.lock.Lock()
		d.eventJobs[event.UID] = len(subscriptions)
		d.lock.Unlock()
		// Fanout all the subscriptions for concurrent execution
		for _, sub := range subscriptions {
			sub.StartedAt = time.Now()
			job := &worker.Job{
				ID:           event.UID + "-" + sub.ID,
				Event:        &event,
				Subscription: sub,
			}
			wrk := d.getAvailableWorker()
			slog.Info("scheduled job", "job_id", job.ID, "event_type", event.EventType, "worker_id", wrk.ID)
			m.RecordDispatchLatency(&event)
			wrk.JobQueue <- job
		}
	}
}

func (d *Dispatcher) Stop() {
	slog.Info("Shutting down dispatcher...")
	for _, worker := range d.Workers {
		worker.Stop()
	}
	slog.Info("jobs processed", "total", d.totalJobs)
}
