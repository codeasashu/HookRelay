package event

import (
	"encoding/json"
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
    VALUES (:id, :owner_id, :event_type, :payload, :idempotency_key, :accepted_at, :created_at, :modified_at)
    `
	payload, err := json.Marshal(s.Payload)
	if err != nil {
		slog.Error("failed to marshal event payload", "err", err)
		return err
	}
	args := map[string]interface{}{
		"id":              s.UID,
		"owner_id":        s.OwnerId,
		"event_type":      s.EventType,
		"payload":         payload,
		"idempotency_key": "",
		"accepted_at":     s.AcknowledgedAt,
		"created_at":      s.CreatedAt,
		"modified_at":     s.CreatedAt,
	}

	slog.Info("creating event", "id", s.UID)

	result, err := r.db.GetDB().NamedExec(
		query,
		args,
	)
	if err != nil {
		slog.Error("DB error creating event11", "err", err)
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
    VALUES (:id, :event_id, :owner_id, :subscription_id, :started_at, :completed_at, :status_code, :error, :latency)
    `
	// VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)

	args := map[string]interface{}{
		"id":              s.ID,
		"event_id":        s.EventID,
		"owner_id":        s.OwnerId,
		"subscription_id": s.SubscriptionId,
		"started_at":      s.StartedAt,
		"completed_at":    s.CompletedAt,
		"status_code":     s.StatusCode,
		"error":           s.Error,
		"latency":         s.Latency,
	}

	slog.Info("creating event delivery", "id", s.ID, "args", fmt.Sprintf("%s, %s, %s, %s, %s, %s, %d, %s, %.2f", s.ID, s.EventID, s.OwnerId, s.SubscriptionId, s.StartedAt, s.CompletedAt, s.StatusCode, s.Error, s.Latency))

	result, err := r.db.GetDB().NamedExec(
		query,
		args,
		// s.ID, s.EventID, s.OwnerId, s.SubscriptionId, s.StartedAt, s.CompletedAt, s.StatusCode, s.Error, s.Latency,
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
