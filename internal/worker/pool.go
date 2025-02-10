package worker

import (
	"errors"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/cli"
)

// @TODO: Make this a linked list to support multiple workers
type WorkerPool struct {
	localClient *LocalClient
	queueClient *QueueClient
}

func NewWorkerPool(app *cli.App) *WorkerPool {
	// Pool always starts with localWorker
	localWorker := NewLocalWorker(app)
	w := &WorkerPool{
		localClient: localWorker.client.(*LocalClient),
	}
	return w
}

func (wp *WorkerPool) AddQueueClient() error {
	qc := NewQueueWorker()
	wp.queueClient = qc.client.(*QueueClient)
	if err := wp.queueClient.client.Ping(); err != nil {
		slog.Warn("remote worker queue is not connected. only local workers will be used", "err", err)
		return err
	}
	slog.Info("remote worker queue is connected")
	return nil
}

func (wp *WorkerPool) ShouldUseRemote(job *Job) bool {
	// Checks if jobs should be scheduled to remote worker
	// based on several criterias
	localWorkerFull := wp.localClient != nil && wp.localClient.IsNearlyFull()
	remoteIsReady := wp.queueClient != nil
	// remoteIsReady := wp.queueClient != nil && wp.queueClient.IsReady()
	return (localWorkerFull && remoteIsReady) || (job.isRetrying && remoteIsReady)
}

func (wp *WorkerPool) Schedule(job *Job) error {
	job.wp = wp
	if wp.ShouldUseRemote(job) {
		slog.Info("scheduling job to queue worker", "job", job)
		return wp.queueClient.SendJob(job)
	}
	if wp.localClient != nil {
		slog.Info("scheduling job to local worker", "job", job)
		return wp.localClient.SendJob(job)
	}
	return errors.New("error scheduling job. no worker available")
}

func (wp *WorkerPool) Retry(job *Job) error {
	if wp.localClient != nil && !wp.localClient.IsNearlyFull() {
		slog.Info("scheduling job to local worker", "job", job)
		return wp.localClient.SendJob(job)
	} else if wp.queueClient != nil && wp.queueClient.IsReady() {
		slog.Info("scheduling job to queue worker", "job", job)
		return wp.queueClient.SendJob(job)
	} else {
		return errors.New("error scheduling job. no worker available")
	}
}
