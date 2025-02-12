package dispatcher

import (
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/worker"

	"github.com/codeasashu/HookRelay/internal/subscription"
)

var m *metrics.Metrics

type Dispatcher struct {
	lock       *sync.RWMutex
	workerPool *worker.WorkerPool
}

func NewDispatcher(wp *worker.WorkerPool) *Dispatcher {
	m = metrics.GetDPInstance()
	return &Dispatcher{
		lock:       &sync.RWMutex{},
		workerPool: wp,
	}
}

func (d *Dispatcher) ListenForEvents(eventChannel <-chan event.Event) {
	app := cli.GetAppInstance()

	subModel := subscription.NewSubscriptionModel(app.DB)
	for event := range eventChannel {
		slog.Info("dispatching event", "id", event.UID, "type", event.EventType)
		subscriptions, err := subModel.FindSubscriptionsByEventTypeAndOwner(event.EventType, event.OwnerId)
		if err != nil {
			slog.Error("error fetching subscriptions", "err", err)
			event.CompletedAt = time.Now()
			continue
		}

		slog.Info("fetched subscriptions", "event_id", event.UID, "fanout", len(subscriptions), "event_type", event.EventType, "owner_id", event.OwnerId)
		m.RecordPreFlightLatency(&event)
		if len(subscriptions) == 0 {
			event.CompletedAt = time.Now()
			continue
		}
		// d.lock.Lock()
		// d.eventJobs[event.UID] = len(subscriptions)
		// d.lock.Unlock()
		// Fanout all the subscriptions for concurrent execution
		for _, sub := range subscriptions {
			sub.StartedAt = time.Now()
			job := &worker.Job{
				ID:           strconv.Itoa(int(event.UID)) + "_" + strings.ToLower(event.EventType),
				Event:        &event,
				Subscription: &sub,
			}
			err := d.workerPool.Schedule(job)
			if err != nil {
				slog.Error("error scheduling job", "job_id", job.ID, "err", err)
				continue
			}
			// wrk := d.getAvailableWorker()
			// slog.Info("scheduled job", "job_id", job.ID, "event_type", event.EventType, "worker_id", wrk.ID)
			// m.RecordDispatchLatency(&event)
			// wrk.JobQueue <- job
		}
	}
}

// func (d *Dispatcher) Stop() {
// 	slog.Info("Shutting down dispatcher...")
// 	for _, worker := range d.Workers {
// 		worker.Stop()
// 	}
// 	slog.Info("jobs processed", "total", d.totalJobs)
// }
