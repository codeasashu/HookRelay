package publisher

import (
	"log/slog"
	"time"

	"github.com/codeasashu/HookRelay/internal/delivery"
	"github.com/codeasashu/HookRelay/internal/wal"
	"github.com/codeasashu/HookRelay/internal/worker"
)

func LogBatchEventDelivery(w wal.AbstractWAL, events []worker.Task) error {
	if len(events) == 0 {
		return nil
	}

	if w == nil {
		return nil
	}

	mu := w.GetMutex()
	mu.Lock()
	defer mu.Unlock()

	db := w.GetDB()

	tx, err := db.Begin()
	if err != nil {
		slog.Error("failed to start transaction for batch event logging", slog.Any("error", err))
		return err
	}

	stmt, err := tx.Prepare(`
        INSERT INTO event_deliveries (event_type, payload, owner_id, subscription_id, status_code, error, start_at, complete_at)
        VALUES (?,?,?,?,?,?,?,?)
    `)
	if err != nil {
		slog.Error("failed to prepare statement for batch event logging", slog.Any("error", err))
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, t := range events {
		e := t.(*delivery.EventDelivery)
		_, err := stmt.Exec(
			e.EventType, e.Payload, e.OwnerId, e.SubscriptionId, e.StatusCode, e.Error, e.StartAt, e.CompleteAt,
		)
		if err != nil {
			slog.Error("failed to log event in batch WAL insert", slog.Any("error", err))
			_ = tx.Rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit batch WAL insert", slog.Any("error", err))
		return err
	}

	slog.Debug("batch logged events in WAL", "count", len(events))
	return nil
}

func ProcessWALBatch(wl wal.AbstractWAL, jobChan <-chan worker.Task) {
	const batchSize = 100
	const flushInterval = 500 * time.Millisecond

	var batch []worker.Task
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case job, ok := <-jobChan:
			if !ok {
				// Channel closed, flush remaining events
				if len(batch) > 0 {
					LogBatchEventDelivery(wl, batch)
				}
				return
			}

			// m.IncrementIngestConsumedTotal(job.Event, "local")
			batch = append(batch, job)

			if len(batch) >= batchSize {
				LogBatchEventDelivery(wl, batch)
				batch = nil // Reset batch
			}
		case <-ticker.C:
			if len(batch) > 0 {
				LogBatchEventDelivery(wl, batch)
				batch = nil // Reset batch
			}
		}
	}
}

func WALPublisher(wl wal.AbstractWAL, f *worker.FanOut) error {
	ch, err := f.Subscribe("wal", 1024)
	if err != nil {
		slog.Error("error starting SQS billing module", "err", err)
		return err
	}
	go func() {
		defer f.Unsubscribe("wal")
		ProcessWALBatch(wl, ch)
	}()
	return nil
}
