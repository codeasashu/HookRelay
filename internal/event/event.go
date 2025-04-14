package event

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
)

type Event struct {
	UID            int64       `json:"uid"`
	Payload        interface{} `json:"payload"`
	OwnerId        string      `json:"owner_id"`
	EventType      string      `json:"event_type"`
	IdempotencyKey string      `json:"idempotency_key"`
	CreatedAt      time.Time
	AcknowledgedAt time.Time
	PayloadBytes   []byte

	TraceId string // Internal field to track event
}

func New() *Event {
	e := &Event{
		UID:       -1,
		CreatedAt: time.Now(),
	}
	return e
}

func NewFromHTTP(req *http.Request) (*Event, error) {
	if req == nil {
		return nil, errors.New("empty HTTP request")
	}
	decoder := json.NewDecoder(req.Body)
	ev := &Event{
		UID:       -1,
		CreatedAt: time.Now(),
		TraceId:   req.Header.Get(config.TraceIDHeaderName),
	}
	err := decoder.Decode(ev)
	if err != nil {
		slog.Error("failed to decode event payload", "trace_id", ev.TraceId, "error", err.Error(), "body", fmt.Sprintf("%+v", ev.Payload))
		return nil, errors.New("failed to decode event payload. invalid json")
	}

	slog.Info("event ready", "trace_id", ev.TraceId, "body", fmt.Sprintf("%+v", ev.Payload))
	return ev, nil
}

func (e *Event) Ack() {
	e.AcknowledgedAt = time.Now()
	slog.Info("received event", "traceid", e.TraceId, "id", e.UID, "type", e.EventType)
	slog.Debug("received event payload", "traceid", e.TraceId, "payload", e.Payload)
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
