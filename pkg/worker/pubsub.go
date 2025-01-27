package worker

import (
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/worker"
)

func StartPubsubWorker() {
	slog.Info("staring pubsub worker")
	jobChannel := make(chan *worker.Job, config.HRConfig.PubsubWorker.QueueSize) // Buffered channel for queuing jobs
	w := worker.NewPubsubWorker()
	// Start a pool of goroutines to process jobs
	slog.Info("staring pool of child workers", "children", config.HRConfig.PubsubWorker.Threads)
	for i := 0; i < config.HRConfig.PubsubWorker.Threads; i++ {
		go worker.ProcessJobFromSubscribedChan(jobChannel)
	}

	// Listen for jobs from Redis and queue them
	w.ReceiveJob(jobChannel)
}
