package event

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"
)

type Event struct {
	UID            int64       `json:"uid"`
	Payload        interface{} `json:"payload"`
	OwnerId        string      `json:"owner_id"`
	EventType      string      `json:"event_type"`
	IdempotencyKey string      `json:"idempotency_key"`
	CreatedAt      time.Time
	AcknowledgedAt time.Time
}

func New() *Event {
	e := &Event{
		UID:       -1,
		CreatedAt: time.Now(),
	}
	return e
}

func (e *Event) Ack() {
	e.AcknowledgedAt = time.Now()
}

func (e *Event) LogEvent(db *sql.DB) error {
	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		payloadBytes = []byte("{}")
	}

	if res, err := db.Exec(`
	        INSERT INTO events (owner_id, event_type, payload, idempotency_key)
	        VALUES (?,?,?,?)`,
		e.OwnerId, e.EventType, payloadBytes, e.IdempotencyKey,
	); err != nil {
		slog.Error("failed to log event in WAL", slog.Any("error", err))
		return err
	} else {
		id, err := res.LastInsertId()
		if err == nil {
			e.UID = id
		} else {
			e.UID = -1
		}
		slog.Debug("logged event in WAL", slog.Any("id", e.UID))
	}
	return nil
}
