package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/hibiken/asynq"

	"github.com/oklog/ulid/v2"
)

const (
	TypeEventDelivery = "event:deliver"
	QueueName         = "hookrelay"
)

type (
	UnmarshalerMap map[string]func([]byte) (Task, error)
	MarshalerMap   map[string]func(Task) ([]byte, error)
)

type QueueWorker struct {
	ID     string
	ctx    context.Context
	client *asynq.Client

	marshalers MarshalerMap
}

func NewQueueWorker(f *app.HookRelayApp, marshalers MarshalerMap) *QueueWorker {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     f.Cfg.QueueWorker.Addr,
		DB:       f.Cfg.QueueWorker.Db,
		Password: f.Cfg.QueueWorker.Password,
		Username: f.Cfg.QueueWorker.Username,
	})

	slog.Info("readying remote queue")
	return &QueueWorker{
		ID:         ulid.Make().String(),
		ctx:        context.Background(),
		client:     client,
		marshalers: marshalers,
	}
}

func (c *QueueWorker) IsReady() bool {
	if c.client != nil {
		if err := c.client.Ping(); err == nil {
			return true
		}
	}
	return false
}

func (c *QueueWorker) Ping() error {
	return c.client.Ping()
}

func (c *QueueWorker) Shutdown() {
	if c.client != nil {
		c.client.Close()
	}
}

func (c *QueueWorker) Enqueue(job Task) error {
	payload, err := c.marshalers[TypeEventDelivery](job)
	if err != nil {
		slog.Error("error creating queue task", "err", err)
		return err
	}
	t := asynq.NewTask(TypeEventDelivery, payload)
	info, err := c.client.Enqueue(t, asynq.Queue(QueueName))
	if err != nil {
		slog.Error("could not enqueue task", "err", err)
		return err
	}
	slog.Info("enqueued task", "task_id", info.ID, "queue", info.Queue)
	return nil
}

func NewQueueJob(job Task) (*asynq.Task, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeEventDelivery, payload), nil
}

func HandleQueueJob(ctx context.Context, t *asynq.Task, unmarshalerMap UnmarshalerMap, callback func([]Task) error) error {
	if unmarshaler, ok := unmarshalerMap[t.Type()]; ok {
		j, err := unmarshaler(t.Payload())
		if err != nil {
			return err
		}

		slog.Info("Processing job", "job_id", j, "event_id", j)
		exeErr := j.Execute() // Update job result

		callback([]Task{j})
		if exeErr != nil {
			slog.Error("error processing job", "job_id", j, "error", err)
			return err
		}
		slog.Info("job complete. sending result", "job_id", j)
		return nil
	}
	return errors.ErrUnsupported
}
