package worker

import (
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

type WorkerClient interface {
	SendJob(job *Job) error
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
