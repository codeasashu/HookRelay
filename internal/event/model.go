package event

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/database"
)

type EventModel struct {
	db database.Database
}

var ErrEventNotCreated = errors.New("subscription could not be created")

func NewEventModel(db database.Database) *EventModel {
	return &EventModel{db: db}
}

func (r *EventModel) CreateEvent(s *Event) error {
	query := `
    INSERT INTO hookrelay.event (id, owner_id, event_type, payload, idempotency_key, accepted_at, created_at, modified_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `

	slog.Info("creating event", "id", s.UID)

	result, err := r.db.GetDB().Exec(
		query,
		s.UID, s.OwnerId, s.EventType, s.Payload, "", s.AcknowledgedAt, s.CreatedAt, s.CreatedAt,
	)
	if err != nil {
		slog.Error("DB error creating event", "err", err)
		return ErrEventNotCreated
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventNotCreated
	}

	return nil
}

func (r *EventModel) CreateEventDelivery(s *EventDelivery) error {
	query := `
    INSERT INTO hookrelay.event_delivery (id, event_id, owner_id, subscription_id, started_at, completed_at, status_code, error, latency)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `

	slog.Info("creating event delivery", "id", s.ID, "args", fmt.Sprintf("%s, %s, %s, %s, %s, %s, %d, %s, %.2f", s.ID, s.EventID, s.OwnerId, s.SubscriptionId, s.StartedAt, s.CompletedAt, s.StatusCode, s.Error, s.Latency))

	result, err := r.db.GetDB().Exec(
		query,
		s.ID, s.EventID, s.OwnerId, s.SubscriptionId, s.StartedAt, s.CompletedAt, s.StatusCode, s.Error, s.Latency,
	)
	if err != nil {
		slog.Error("DB error creating event delivery", "err", err)
		return ErrEventNotCreated
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEventNotCreated
	}

	return nil
}
