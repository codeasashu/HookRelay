package delivery

import (
	"log"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/worker"
)

func SaveBatchEventDelivery(db database.Database, deliveries []worker.Task) error {
	if len(deliveries) == 0 {
		return nil
	}

	// Skip delivery if delivery DB is not set
	tx, err := db.GetDB().Beginx()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Preparex(`
        INSERT INTO hookrelay.event_delivery 
        (event_type, payload, owner_id, subscription_id, status_code, error, start_at, complete_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	for _, d := range deliveries {
		ed := d.(*EventDelivery)
		_, err := stmt.Exec(
			ed.EventType,
			ed.Payload,
			ed.OwnerId,
			ed.SubscriptionId,
			ed.StatusCode,
			ed.Error,
			ed.StartAt.UTC().Format("2006-01-02 15:04:05.999999"),
			ed.CompleteAt.UTC().Format("2006-01-02 15:04:05.999999"),
		)
		if err != nil {
			slog.Error("Error inserting to MySQL:", "err", err)
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit batch WAL insert", slog.Any("error", err))
		return err
	}

	slog.Debug("batch logged events in DB", "count", len(deliveries))
	return nil
}
