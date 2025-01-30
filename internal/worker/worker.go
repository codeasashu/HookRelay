package worker

import (
	"log/slog"
	"sync"

	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/subscription"

	"github.com/oklog/ulid/v2"
)

var m *metrics.Metrics

type Job struct {
	ID           string
	Event        *event.Event
	Subscription *subscription.Subscription
	Result       *event.EventDelivery
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
	ID     string
	client WorkerClient
}

func NewWorker() *Worker {
	return &Worker{
		ID: ulid.Make().String(),
	}
}

func (w *Worker) ReceiveJob(onChan chan *Job) {
	if w.client != nil {
		w.client.ReceiveJob(onChan)
	}
}

func (w *Worker) DispatchJob(job *Job) error {
	slog.Info("sending job", "client", w.client, "worker", job.ID)
	if w.client != nil {
		w.client.SendJob(job)
	} else {
		slog.Error("error sending job", "client", w.client, "worker", job.ID)
	}
	return nil
}
