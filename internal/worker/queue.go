package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/hibiken/asynq"

	"github.com/oklog/ulid/v2"
)

const TypeEventDelivery = "event:deliver"

//	func NewQueue() *asynq.Client {
//		return asynq.NewClient(asynq.RedisClientOpt{
//			Addr:     config.HRConfig.QueueWorker.Addr,
//			DB:       config.HRConfig.QueueWorker.Db,
//			Password: config.HRConfig.QueueWorker.Password,
//			Username: config.HRConfig.QueueWorker.Username,
//		})
//	}

type QueueClient struct {
	ctx    context.Context
	client *asynq.Client
}

func NewQueueWorker() *Worker {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     config.HRConfig.QueueWorker.Addr,
		DB:       config.HRConfig.QueueWorker.Db,
		Password: config.HRConfig.QueueWorker.Password,
		Username: config.HRConfig.QueueWorker.Username,
	})

	return &Worker{
		ID: ulid.Make().String(),
		client: &QueueClient{
			ctx:    context.Background(),
			client: client,
		},
	}
}

func (c *QueueClient) SendJob(job *Job) error {
	t, err := NewQueueJob(job)
	if err != nil {
		slog.Error("error creating redis task", "err", err)
		return err
	}
	info, err := c.client.Enqueue(t, asynq.Queue("hookrelay"))
	if err != nil {
		slog.Error("could not enqueue task", "err", err)
		return err
	}
	slog.Info("enqueued task", "task_id", info.ID, "queue", info.Queue)
	return nil
}

func (c *QueueClient) ReceiveJob(jobChannel chan<- *Job) {
}

func NewQueueJob(job *Job) (*asynq.Task, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeEventDelivery, payload), nil
}

func HandleQueueJob(ctx context.Context, t *asynq.Task) error {
	app := cli.GetAppInstance()
	var j Job
	m := ctx.Value(metrics.MetricsContextKey).(*metrics.Metrics)
	if err := json.Unmarshal(t.Payload(), &j); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	m.RecordDispatchLatency(j.Event)
	slog.Info("Processing job", "job_id", j.ID, "event_id", j.Event.UID)
	// err := processJob(&j)
	statusCode, err := j.Subscription.Target.ProcessTarget(j.Event.Payload)
	m.IncrementIngestConsumedTotal(j.Event)
	if err != nil {
		m.IncrementIngestSuccessTotal(j.Event)
	} else {
		m.IncrementIngestErrorsTotal(j.Event)
	}

	j.Result = event.NewEventDelivery(j.Event, j.Subscription.ID, statusCode, err)
	deliveryModel := event.NewEventModel(app.DB)
	deliveryModel.CreateEventDelivery(j.Result)
	slog.Info("Finished Processing job", "job_id", j.ID, "event_id", j.Event.UID)
	m.RecordEndToEndLatency(j.Result)
	return nil
}
