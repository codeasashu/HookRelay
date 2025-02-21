package delivery

import (
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/wal"
	"github.com/codeasashu/HookRelay/internal/worker"
)

func LogEventDelivery(w wal.AbstractWAL, t worker.Task) error {
	mu := w.GetMutex()
	mu.Lock()
	defer mu.Unlock()

	db := w.GetDB()

	e := t.(*EventDelivery)

	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		payloadBytes = []byte("{}")
	}

	if res, err := db.Exec(
		`
        INSERT INTO event_deliveries
        (event_type, payload, owner_id, subscription_id, status_code, error, start_at, complete_at)
        VALUES (?,?,?,?,?,?,?,?)`,
		e.EventType, payloadBytes, e.OwnerId, e.SubscriptionId, e.StatusCode, e.Error, e.StartAt, e.CompleteAt,
	); err != nil {
		slog.Error("failed to log event delivery in WAL", slog.Any("error", err))
		return err
	} else {
		id, err := res.LastInsertId()
		if err == nil {
			e.ID = id
		} else {
			e.ID = -1
		}
		slog.Debug("logged event delivery in WAL", slog.Any("id", e.ID))
	}
	return nil
}

func LogBatchEventDelivery(w wal.AbstractWAL, events []worker.Task) error {
	if len(events) == 0 {
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
		e := t.(*EventDelivery)
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

func ProcessRotatedWAL(deliveryDb database.Database) func(db *sql.DB) error {
	batchSize := 100
	return func(db *sql.DB) error {
		rows, err := db.Query(`
            SELECT event_type, payload, owner_id, subscription_id, status_code, error, start_at, complete_at
            FROM event_deliveries
        `)
		if err != nil {
			return err
		}

		batch := make([]worker.Task, 0, batchSize)
		processedDeliveries := 0
		for rows.Next() {
			var e EventDelivery
			slog.Info("processing recovery WAL", "delivery", e.ID)

			if err := rows.Scan(
				&e.EventType, &e.Payload, &e.OwnerId, &e.SubscriptionId, &e.StatusCode, &e.Error, &e.StartAt, &e.CompleteAt,
			); err != nil {
				slog.Error("failed to scan WAL", slog.Any("error", err))
				continue
			}

			batch = append(batch, worker.Task(&e))
			processedDeliveries++

			// Process batch if it reaches the batch size
			if len(batch) >= batchSize {
				slog.Debug("processing batch", slog.Int("batch_size", len(batch)), slog.Int("processed", processedDeliveries))
				if err := SaveBatchEventDelivery(deliveryDb, batch); err != nil {
					rows.Close()
					return err
				}
				batch = batch[:0] // Reset batch
			}
		}

		// Process any remaining events in the batch
		if len(batch) > 0 {
			slog.Debug("processing final batch", slog.Int("batch_size", len(batch)), slog.Int("processed", processedDeliveries))
			if err := SaveBatchEventDelivery(deliveryDb, batch); err != nil {
				rows.Close()
				return err
			}
		}

		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}

		rows.Close()
		return nil
	}
}
