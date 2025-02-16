package worker

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/wal"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/hibiken/asynq"
)

type AsynqWorker struct {
	ctx           context.Context
	asyncqServer  *asynq.Server
	metricsServer *http.Server
}

func metricsWrapper() func(asynq.Handler) asynq.Handler {
	mw := metrics.GetDPInstance()
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			newctx := context.WithValue(ctx, metrics.MetricsContextKey, mw)
			return next.ProcessTask(newctx, t)
		})
	}
}

func StartMetricsServer() *http.Server {
	httpServeMux := http.NewServeMux()
	httpServeMux.Handle("/metrics", metrics.GetHandler())
	metricsSrv := &http.Server{
		Addr:    config.HRConfig.Metrics.WorkerAddr,
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

func HandleJobWithAccounting(accounting *wal.Accounting) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		return worker.HandleQueueJob(ctx, t, accounting)
	}
}

func (q *AsynqWorker) Shutdown() {
	q.asyncqServer.Shutdown()
	if err := q.metricsServer.Shutdown(q.ctx); err != nil {
		slog.Error("Error: metrics server shutdown", "err", err)
	}
}

func StartQueueWorker(ctx context.Context, accounting *wal.Accounting) (*AsynqWorker, error) {
	metricSrv := StartMetricsServer()
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     config.HRConfig.QueueWorker.Addr,
			DB:       config.HRConfig.QueueWorker.Db,
			Password: config.HRConfig.QueueWorker.Password,
			Username: config.HRConfig.QueueWorker.Username,
		},
		asynq.Config{
			Concurrency: config.HRConfig.QueueWorker.Concurrency,
			Queues: map[string]int{
				worker.QueueName:       9,
				worker.ResultQueueName: 1,
			},
		},
	)

	asynqWorker := &AsynqWorker{
		ctx:           ctx,
		asyncqServer:  srv,
		metricsServer: metricSrv,
	}

	mux := asynq.NewServeMux()
	mux.Use(metricsWrapper())
	mux.HandleFunc(worker.TypeEventDelivery, HandleJobWithAccounting(accounting))

	// Start worker server.
	if err := srv.Start(mux); err != nil {
		slog.Error("Failed to start worker server, shutting down", "err", err)
		metricSrv.Shutdown(ctx)
		return nil, err
	}

	return asynqWorker, nil
}
