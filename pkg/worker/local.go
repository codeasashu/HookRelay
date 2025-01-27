package worker

import (
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/worker"
)

var m *metrics.Metrics

func StartLocalWorker() *worker.Worker {
	m = metrics.GetDPInstance()
	slog.Info("staring local worker")
	jobResultChan := make(chan *worker.Job, config.HRConfig.LocalWorker.QueueSize/2) // Buffered channel for queuing jobs
	w := worker.NewLocalWorker()
	// Start a pool of goroutines to process jobs
	slog.Info("staring pool of child workers", "children", config.HRConfig.LocalWorker.ResultHandlerThreads)
	for i := 0; i < config.HRConfig.LocalWorker.ResultHandlerThreads; i++ {
		go worker.ProcessResultsFromLocalChan(jobResultChan)
	}

	// Listen for jobs from Redis and queue them
	w.ReceiveJob(jobResultChan)
	return w
}
