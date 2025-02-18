package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/wal"
	"github.com/hibiken/asynq"

	"github.com/oklog/ulid/v2"
)

const (
	TypeEventDelivery = "event:deliver"
	QueueName         = "hookrelay"
)

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

	slog.Info("readying remote queue")
	return &Worker{
		ID: ulid.Make().String(),
		client: &QueueClient{
			ctx:    context.Background(),
			client: client,
		},
	}
}

func (c *QueueClient) IsReady() bool {
	if c.client != nil {
		if err := c.client.Ping(); err == nil {
			return true
		}
	}
	return false
}

func (c *QueueClient) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

func (c *QueueClient) SendJob(job *Job) error {
	t, err := NewQueueJob(job)
	if err != nil {
		slog.Error("error creating redis task", "err", err)
		return err
	}
	info, err := c.client.Enqueue(
		t, asynq.Queue(QueueName), asynq.MaxRetry(int(job.Subscription.Target.MaxRetries-job.numDeliveries)-1),
	)
	if err != nil {
		slog.Error("could not enqueue task", "err", err)
		return err
	}
	slog.Info("enqueued task", "task_id", info.ID, "queue", info.Queue)
	return nil
}

func NewQueueJob(job *Job) (*asynq.Task, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeEventDelivery, payload), nil
}

func HandleQueueJob(ctx context.Context, t *asynq.Task, accounting *wal.Accounting) error {
	var j Job
	m := ctx.Value(metrics.MetricsContextKey).(*metrics.Metrics)
	if err := json.Unmarshal(t.Payload(), &j); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	m.RecordDispatchLatency(j.Event, "asynq")
	slog.Info("Processing job", "job_id", j.ID, "event_id", j.Event.UID)
	_, err := j.Exec() // Update job result
	m.RecordEndToEndLatency(j.Event, "asynq")

	// Add event delivery to result processing queue
	// @TODO: Batch the inserts or use channels
	accounting.CreateDeliveries([]*event.EventDelivery{j.Result})
	// wl.LogEventDelivery(j.Result)
	if err != nil {
		slog.Error("error processing job", "job_id", j.ID, "error", err)
		return err
	}
	slog.Info("job complete. sending result", "job_id", j.ID)
	return nil
}
