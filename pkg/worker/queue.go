package worker

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/delivery"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/hibiken/asynq"
)

type AsynqWorker struct {
	ctx           context.Context
	asyncqServer  *asynq.Server
	metricsServer *http.Server
}

func metricsWrapper(f *app.HookRelayApp) func(asynq.Handler) asynq.Handler {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			newctx := context.WithValue(ctx, metrics.MetricsContextKey, f.Metrics)
			return next.ProcessTask(newctx, t)
		})
	}
}

func StartMetricsServer(f *app.HookRelayApp) *http.Server {
	httpServeMux := http.NewServeMux()
	httpServeMux.Handle("/metrics", metrics.GetHandler())
	metricsSrv := &http.Server{
		Addr:    f.Cfg.Metrics.WorkerAddr,
		Handler: httpServeMux,
	}

	// Start metrics server.
	go func() {
		err := metricsSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Error: metrics server error", "err", err)
		}
	}()

	return metricsSrv
}

func HandleJobWithAccounting(f *app.HookRelayApp, unmarshalerMap worker.UnmarshalerMap) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		slog.Info("handling queue job")
		err := worker.HandleQueueJob(ctx, t, unmarshalerMap, delivery.SaveDeliveries(f))
		slog.Info("handled queue job", "err", err)
		return err
	}
}

func (q *AsynqWorker) Shutdown() {
	q.asyncqServer.Shutdown()
	if err := q.metricsServer.Shutdown(q.ctx); err != nil {
		slog.Error("Error: metrics server shutdown", "err", err)
	}
}

func StartQueueWorker(f *app.HookRelayApp, unmarshalerMap worker.UnmarshalerMap) (*AsynqWorker, error) {
	metricSrv := StartMetricsServer(f)
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     f.Cfg.QueueWorker.Addr,
			DB:       f.Cfg.QueueWorker.Db,
			Password: f.Cfg.QueueWorker.Password,
			Username: f.Cfg.QueueWorker.Username,
		},
		asynq.Config{
			Concurrency: f.Cfg.QueueWorker.Concurrency,
			Queues: map[string]int{
				worker.QueueName: 10,
			},
		},
	)

	asynqWorker := &AsynqWorker{
		ctx:           f.Ctx,
		asyncqServer:  srv,
		metricsServer: metricSrv,
	}

	mux := asynq.NewServeMux()
	mux.Use(metricsWrapper(f))
	mux.HandleFunc(worker.TypeEventDelivery, HandleJobWithAccounting(f, unmarshalerMap))

	// Start worker server.
	if err := srv.Start(mux); err != nil {
		slog.Error("Failed to start worker server, shutting down", "err", err)
		metricSrv.Shutdown(f.Ctx)
		return nil, err
	}

	return asynqWorker, nil
}
