package worker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/disruptor"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/oklog/ulid/v2"
)

const (
	BufferSize   = 1024 * 64
	BufferMask   = BufferSize - 1
	Iterations   = 128 * 1024 * 32
	Reservations = 1
)

var ringBuffer [BufferSize]*Job

type LocalClient struct {
	ID            string
	app           *cli.App
	ctx           context.Context
	client        string
	ruptor        *disruptor.Disruptor
	JobQueue      chan *Job
	resultQueue   chan<- *Job
	MaxThreads    int            // Maximum allowed threads
	MinThreads    int            // Minimum threads to keep alive
	activeThreads int32          // Active thread count (atomic counter)
	StopChan      chan struct{}  // Signal for stopping the worker
	wg            sync.WaitGroup // WaitGroup for graceful shutdown
	metrics       *metrics.Metrics
}

type LocalConsumer struct {
	client *LocalClient
}

var localDisruptor disruptor.Disruptor

func NewLocalWorker(app *cli.App) *Worker {
	minThreads := config.HRConfig.LocalWorker.MinThreads
	maxThreads := config.HRConfig.LocalWorker.MaxThreads
	if maxThreads != -1 && maxThreads < minThreads {
		slog.Warn("max threads less than min thread. updating", "from", maxThreads, "to", minThreads)
		maxThreads = minThreads
	}

	m = metrics.GetDPInstance()
	jobResultChan := make(chan *Job, config.HRConfig.LocalWorker.QueueSize/2) // Buffered channel for queuing jobs
	client := &LocalClient{
		ctx:         context.Background(),
		app:         app,
		client:      "",
		JobQueue:    make(chan *Job, config.HRConfig.LocalWorker.QueueSize/2), // Buffer size for job queue
		MaxThreads:  maxThreads,
		MinThreads:  minThreads,
		StopChan:    make(chan struct{}),
		metrics:     m,
		resultQueue: jobResultChan,
	}

	w := &Worker{
		ID:     ulid.Make().String(),
		client: client,
	}
	consumer := LocalConsumer{
		client: client,
	}

	localDisruptor = disruptor.New(
		disruptor.WithCapacity(BufferSize),
		disruptor.WithConsumerGroup(consumer),
	)

	go localDisruptor.Read()

	slog.Info("staring pool of local workers", "children", config.HRConfig.LocalWorker.ResultHandlerThreads)
	for i := 0; i < config.HRConfig.LocalWorker.ResultHandlerThreads; i++ {
		go ProcessResultsFromLocalChan(jobResultChan)
	}

	return w
}

func (c LocalConsumer) Consume(lowerSequence, upperSequence int64) {
	for sequence := lowerSequence; sequence <= upperSequence; sequence++ {
		index := sequence & BufferMask
		job := ringBuffer[index]

		slog.Info("got job item", "job_id", job.ID)
		c.client.metrics.RecordDispatchLatency(job.Event)
		job.Subscription.Dispatch()
		statusCode, err := job.Subscription.Target.ProcessTarget(job.Event.Payload)
		job.Subscription.Complete()
		slog.Info("job complete. sending result", "job_id", job.ID)
		job.Result = event.NewEventDelivery(job.Event, job.Subscription.ID, statusCode, err)
		// @TODO: Dont insert delivery result right away, process in batch (maybe insert into redis)
		// deliveryModel := event.NewEventModel(app.DB)
		// deliveryModel.CreateEventDelivery(job.Result)
		c.client.resultQueue <- job
	}
}

func (c *LocalClient) SendJob(job *Job) error {
	// c.JobQueue <- job
	sequence := localDisruptor.Reserve(1)
	ringBuffer[sequence&BufferMask] = job
	localDisruptor.Commit(sequence, sequence)
	slog.Info("Sending job", "job", job)
	return nil
}

func (c *LocalClient) ReceiveJob(jobChannel chan<- *Job) {
	c.resultQueue = jobChannel
	for i := 0; i < c.MinThreads; i++ {
		c.launchThread()
	}
	go c.scaleThreads(1 * time.Second)
}

func (c *LocalClient) scaleThreads(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C: // Periodic check
			queueLen := len(c.JobQueue)
			active := atomic.LoadInt32(&c.activeThreads)
			m.UpdateWorkerQueueSize(c.ID, queueLen)
			m.UpdateWorkerThreadCount(c.ID, int(active))

			if queueLen > 0 && (c.MaxThreads == -1 || active < int32(c.MaxThreads)) {
				// Increase threads if the queue is filling up.
				slog.Debug("increasing worker threas", "worker_id", c.ID)
				c.launchThread()
			} else if queueLen == 0 && active > int32(c.MinThreads) {
				// Reduce threads if the queue is empty.
				slog.Debug("decreasing worker threads", "worker_id", c.ID)
				c.terminateThread()
			}
		case <-c.StopChan:
			return
		}
	}
}

// launchThread starts a new thread to process jobs.
func (w *LocalClient) launchThread() {
	// app := cli.GetAppInstance()
	w.wg.Add(1)
	atomic.AddInt32(&w.activeThreads, 1)
	m.UpdateWorkerThreadCount(w.ID, int(w.activeThreads))

	slog.Debug("Increased Worker threads by 1", "worker_id", w.ID, "current_count", w.activeThreads)
	go func() {
		defer func() {
			w.wg.Done()
			atomic.AddInt32(&w.activeThreads, -1)
			m.UpdateWorkerThreadCount(w.ID, int(w.activeThreads))
			slog.Debug("Decreased Worker threads by 1", "worker_id", w.ID, "current_count", w.activeThreads)
		}()

		for {
			select {
			case job := <-w.JobQueue:
				slog.Info("got job item", "job_id", job.ID)
				m.RecordDispatchLatency(job.Event)
				job.Subscription.Dispatch()
				statusCode, err := job.Subscription.Target.ProcessTarget(job.Event.Payload)
				job.Subscription.Complete()
				slog.Info("job complete. sending result", "job_id", job.ID)
				job.Result = event.NewEventDelivery(job.Event, job.Subscription.ID, statusCode, err)
				// @TODO: Dont insert delivery result right away, process in batch (maybe insert into redis)
				// deliveryModel := event.NewEventModel(app.DB)
				// deliveryModel.CreateEventDelivery(job.Result)
				w.resultQueue <- job
			case <-w.StopChan:
				return
			}
		}
	}()
}

// terminateThread stops a thread by signaling a reduction in workload.
func (w *LocalClient) terminateThread() {
	// No specific mechanism to stop a thread; threads exit naturally when StopChan is closed.
	w.StopChan <- struct{}{}
}

// Stop gracefully stops the worker.
func (w *LocalClient) Stop() {
	close(w.StopChan)
	w.wg.Wait()
	close(w.JobQueue)
	close(w.resultQueue)
}

func ProcessResultsFromLocalChan(jobChan <-chan *Job) {
	app := cli.GetAppInstance()
	for job := range jobChan {
		m.IncrementIngestConsumedTotal(job.Event)
		// @TODO: Dont insert delivery result right away, process in batch (maybe insert into redis)
		deliveryModel := event.NewEventModel(app.DB)
		deliveryModel.CreateEventDelivery(job.Result)
		if job.Result.IsSuccess() {
			slog.Info("job completed", "job_id", job.ID)
			m.IncrementIngestSuccessTotal(job.Event)
		} else {
			slog.Error("job failed", "job_id", job.ID)
			m.IncrementIngestErrorsTotal(job.Event)
		}
		m.RecordEndToEndLatency(job.Result)
	}
}
