package wal

import (
	"log"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/event"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

type Accounting struct {
	db database.Database
}

func NewAccounting(db database.Database) *Accounting {
	return &Accounting{
		db: db,
	}
}

func (a *Accounting) CreateDeliveries(deliveries []*event.EventDelivery) {
	tx, err := a.db.GetDB().Beginx()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Preparex("INSERT INTO hookrelay.event_delivery (event_type, payload, subscription_id, status_code, error, start_at, complete_at) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	for _, d := range deliveries {
		_, err := stmt.Exec(
			d.EventType,
			d.Payload,
			d.SubscriptionId,
			d.StatusCode,
			d.Error,
			d.StartAt.UTC().Format("2006-01-02 15:04:05.999999"),
			d.CompleteAt.UTC().Format("2006-01-02 15:04:05.999999"),
		)
		if err != nil {
			slog.Error("Error inserting to MySQL:", "err", err)
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("failed to commit batch WAL insert", slog.Any("error", err))
		return
	}

	slog.Debug("batch logged events in DB", "count", len(deliveries))
}
