package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/hibiken/asynq"
)

const TypeEventDelivery = "event:deliver"

func NewRedisClient() *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     config.HRConfig.RedisQueue.Addr,
		DB:       config.HRConfig.RedisQueue.Db,
		Password: config.HRConfig.RedisQueue.Password,
		Username: config.HRConfig.RedisQueue.Username,
	})
}

func NewWorkerJob(job *Job) (*asynq.Task, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeEventDelivery, payload), nil
}

func HandleWorkerJob(ctx context.Context, t *asynq.Task) error {
	var j Job
	m := ctx.Value(metrics.MetricsContextKey).(*metrics.Metrics)
	if err := json.Unmarshal(t.Payload(), &j); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	m.RecordDispatchLatency(j.Event)
	m.RecordPreFlightLatency(j.Event)
	slog.Info("Processing job", "job_id", j.ID, "event_id", j.Event.UID)
	err := processJob(&j)
	m.IncrementIngestConsumedTotal(j.Event)
	if err != nil {
		m.IncrementIngestSuccessTotal(j.Event)
	} else {
		m.IncrementIngestErrorsTotal(j.Event)
	}
	slog.Info("Finished Processing job", "job_id", j.ID, "event_id", j.Event.UID)
	m.RecordEndToEndLatency(j.Event)
	return nil
}
