package worker

import (
	"errors"

	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/subscription"

	"github.com/oklog/ulid/v2"
)

var (
	ErrTooManyRetry = errors.New("job: too many retries")
	ErrRemoteJob    = errors.New("job: remote job error")
)

var m *metrics.Metrics

type Job struct {
	ID            string
	Event         *event.Event
	Subscription  *subscription.Subscription
	Result        *event.EventDelivery
	numDeliveries uint16
	wp            *WorkerPool
	isRetrying    bool
}

func (j *Job) Retry() error {
	j.isRetrying = true
	if j.numDeliveries > j.Subscription.Target.MaxRetries {
		return ErrTooManyRetry
	}
	if j.wp == nil {
		return ErrRemoteJob
	}
	return j.wp.Schedule(j)
}

func (j *Job) Exec() (*Job, error) {
	if m == nil {
		m = metrics.GetDPInstance()
	}
	statusCode, err := j.Subscription.Target.ProcessTarget(j.Event.Payload)
	j.numDeliveries++
	j.Result = event.NewEventDelivery(j.Event, j.Subscription.ID, statusCode, err)
	if j.Result.IsSuccess() {
		m.IncrementIngestSuccessTotal(j.Event, "local")
	} else {
		m.IncrementIngestErrorsTotal(j.Event, "local")
	}
	return j, err
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
