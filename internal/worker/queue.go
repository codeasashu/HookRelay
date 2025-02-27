package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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
	ID      string
	ctx     context.Context
	metrics *metrics.Metrics

	client     *asynq.Client
	server     *asynq.Server
	metricsSrv *http.Server

	marshalers   MarshalerMap
	unmarshalers UnmarshalerMap
}

func createMetricsServer(m *metrics.Metrics, workerAddr string) *http.Server {
	if m != nil && !m.IsEnabled {
		return nil
	}
	httpServeMux := http.NewServeMux()
	handler := promhttp.HandlerFor(m.Registery, promhttp.HandlerOpts{Registry: m.Registery})
	httpServeMux.Handle("/metrics", handler)
	return &http.Server{
		Addr:    workerAddr,
		Handler: httpServeMux,
	}
}

func NewQueueServer(f *app.HookRelayApp, unmarshalers UnmarshalerMap) *QueueWorker {
	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     f.Cfg.QueueWorker.Addr,
			DB:       f.Cfg.QueueWorker.Db,
			Password: f.Cfg.QueueWorker.Password,
			Username: f.Cfg.QueueWorker.Username,
		},
		asynq.Config{
			Concurrency: f.Cfg.QueueWorker.Concurrency,
			Queues: map[string]int{
				QueueName: 1,
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				return 1 * time.Second
			},
		},
	)

	return &QueueWorker{
		ID:           "asynq-" + ulid.Make().String(),
		ctx:          context.Background(),
		server:       server,
		metricsSrv:   createMetricsServer(f.Metrics, f.Cfg.Metrics.WorkerAddr),
		unmarshalers: unmarshalers,
		metrics:      f.Metrics,
	}
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
		ID:         "asynq-" + ulid.Make().String(),
		ctx:        context.Background(),
		client:     client,
		marshalers: marshalers,
		metrics:    f.Metrics,
	}
}

func (w *QueueWorker) startMetricsServer() {
	if w.metricsSrv == nil {
		return
	}
	// Start metrics server.
	go func() {
		err := w.metricsSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Error: metrics server error", "err", err)
		}
	}()
}

func (c *QueueWorker) StartServer(callback func([]Task) error) error {
	if c.server != nil {
		c.startMetricsServer()

		mux := asynq.NewServeMux()
		mux.HandleFunc(TypeEventDelivery, func(ctx context.Context, t *asynq.Task) error {
			if unmarshaler, ok := c.unmarshalers[t.Type()]; ok {
				j, err := unmarshaler(t.Payload())
				if err != nil {
					return err
				}
				return c.Dequeue(j, callback)
			}
			return errors.ErrUnsupported
		})
		return c.server.Start(mux)
	}
	return errors.New("queue server not set")
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

	if c.server != nil {
		c.server.Shutdown()

		if c.metricsSrv != nil {
			c.metricsSrv.Close()
		}
	}
}

func (c *QueueWorker) Enqueue(job Task) error {
	payload, err := c.marshalers[TypeEventDelivery](job)
	if err != nil {
		slog.Error("error creating queue task", "err", err)
		return err
	}
	t := asynq.NewTask(TypeEventDelivery, payload)

	// Add retry
	retiresLeft := job.Retries() - job.NumDeliveries()
	if retiresLeft < 0 {
		retiresLeft = 0
	}
	info, err := c.client.Enqueue(t, asynq.Queue(QueueName), asynq.MaxRetry(retiresLeft))
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

func (w *QueueWorker) GetMetricsHandler() *metrics.Metrics {
	return w.metrics
}

func (w *QueueWorker) GetID() string {
	return w.ID
}

func (w *QueueWorker) GetType() WorkerType {
	return WorkerType("queue")
}

func (w *QueueWorker) Dequeue(j Task, callback func([]Task) error) error {
	slog.Info("remote worker processing task", "trace_id", j.GetTraceID(), "event_id", j)
	err := j.Execute(w) // Update job result
	// Update total deliveries
	j.IncDeliveries()
	callback([]Task{j})
	if err != nil {
		slog.Error("error processing job", "trace_id", j.GetTraceID(), "error", err)
		return err
	}
	slog.Info("job complete. sending result", "trace_id", j.GetTraceID())
	return nil
}
