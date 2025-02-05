package dispatcher

import (
	"log/slog"
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
	lock    *sync.RWMutex
	Workers []*worker.Worker
}

func NewDispatcher() *Dispatcher {
	m = metrics.GetDPInstance()
	// wrk := worker.NewWorker()
	return &Dispatcher{
		lock:    &sync.RWMutex{},
		Workers: []*worker.Worker{},
	}
}

func (d *Dispatcher) AddQueueWorker() {
	wrk := worker.NewQueueWorker()
	d.Workers = append(d.Workers, wrk)
}

func (d *Dispatcher) AddLocalWorker(wrk *worker.Worker) {
	d.Workers = append(d.Workers, wrk)
}

func (d *Dispatcher) getAvailableWorker() *worker.Worker {
	// @TODO: Make better algo
	if len(d.Workers) > 0 {
		return d.Workers[0]
	}
	return nil
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

		slog.Info("fetched subscriptions", "event_id", event.UID, "fanout", len(subscriptions), "event_type", event.EventType)
		m.RecordPreFlightLatency(&event)
		if len(subscriptions) == 0 {
			event.CompletedAt = time.Now()
			continue
		}
		// Fanout all the subscriptions for concurrent execution
		for _, sub := range subscriptions {
			sub.StartedAt = time.Now()
			job := &worker.Job{
				ID:           event.UID + "-" + sub.ID,
				Event:        &event,
				Subscription: &sub,
			}
			wrk := d.getAvailableWorker()
			err := wrk.DispatchJob(job)
			if err != nil {
				slog.Error("error creating redis task", "err", err)
				continue
			}
		}
	}
}
