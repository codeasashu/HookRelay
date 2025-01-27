package worker

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/hibiken/asynq"
	"golang.org/x/sys/unix"
)

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

func StartQueueWorker(sigs chan os.Signal) *asynq.Server {
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
				"hookrelay": 1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.Use(metricsWrapper())
	mux.HandleFunc(worker.TypeEventDelivery, worker.HandleQueueJob)

	// Start worker server.
	if err := srv.Start(mux); err != nil {
		slog.Error("Failed to start worker server, shutting down", "err", err)
		sigs <- unix.SIGTERM
	}

	return srv
}
