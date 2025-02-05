package subscription

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/target"
)

type SubscriptionStatus int

const (
	SubscriptionDeleted SubscriptionStatus = 0
	SubscriptionActive  SubscriptionStatus = 1
)

type SubscriptionModel struct {
	db database.Database
}

var (
	ErrSubscriptionNotCreated = errors.New("subscription could not be created")
	ErrSubscriptionNotFound   = errors.New("subscription could not be found")
	ErrSubscriptionExists     = errors.New("subscription already exists")
)

func NewSubscriptionModel(db database.Database) *SubscriptionModel {
	return &SubscriptionModel{db: db}
}

func (r *SubscriptionModel) Validate(s *Subscription) error {
	if err := target.ValidateTarget(*s.Target); err != nil {
		slog.Error("error validating subscription target", "err", err, "target", *s.Target)
		return err
	}

	// Existing Subscription does not exist
	if _, err := r.HasSubscriptions(s.ID); err != nil {
		slog.Error("error validating existing subscriptions", "err", err, "id", s.ID, "owner_id", s.OwnerId)
		return err
	}
	return nil
}

func (r *SubscriptionModel) CreateSubscription(s *Subscription) error {
	if err := r.Validate(s); err != nil {
		slog.Error("error validating subscription", "err", err)
		return err
	}

	query := `
    INSERT INTO hookrelay.subscription (id, owner_id, target_url, target_method, target_params, target_auth, event_types, status, filters, tags, created, modified)
    VALUES (:id, :owner_id, :target_url, :target_method, :target_params, :target_auth, :event_types, :status, :filters, :tags, :created, :modified)
    `
	// VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)

	eventTypes, err := json.Marshal(s.EventTypes)
	if err != nil {
		slog.Error("failed to marshal subscription event types", "err", err)
		return err
	}

	tags, err := json.Marshal(s.Tags)
	if err != nil {
		slog.Error("failed to marshal subscription tags", "err", err)
		return err
	}

	args := map[string]interface{}{
		"id":            s.ID,
		"owner_id":      s.OwnerId,
		"target_url":    s.Target.HTTPDetails.URL,
		"target_method": s.Target.HTTPDetails.Method,
		"target_params": "[]",
		"target_auth":   "{}",
		"event_types":   eventTypes,
		"status":        int(SubscriptionActive),
		"filters":       "[]",
		"tags":          tags,
		"created":       s.CreatedAt,
		"modified":      s.CreatedAt,
	}

	slog.Info("creating subscription", "id", s.ID)

	result, err := r.db.GetDB().NamedExec(
		query,
		args,
		// s.ID, s.OwnerId, s.Target.HTTPDetails.URL, s.Target.HTTPDetails.Method, "[]", "{}", eventTypes, int(SubscriptionActive), "[]", tags, s.CreatedAt, s.CreatedAt,
	)
	if err != nil {
		slog.Error("DB error creating subscription", "err", err)
		return ErrSubscriptionNotCreated
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrSubscriptionNotCreated
	}

	return nil
}

func (r *SubscriptionModel) FindSubscriptionsByOwner(ownerID string) ([]*Subscription, error) {
	query := `
    SELECT * FROM hookrelay.subscription WHERE owner_id = :owner_id
    `

	rows, err := r.db.GetDB().NamedQuery(query, map[string]interface{}{"owner_id": ownerID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSubscriptionNotFound
		}
		return nil, err
	}
	defer rows.Close()

	var subscriptions []*Subscription
	for rows.Next() {
		var sub *Subscription
		if err := rows.StructScan(&sub); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, sub)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (r *SubscriptionModel) FindSubscriptionsByEventTypeAndOwner(eventType, ownerID string) ([]Subscription, error) {
	var query string
	if config.HRConfig.Database.Type == config.PostgresDatabaseProvider {
		query = `
    SELECT id, owner_id, target_url, target_method, target_params, target_auth, event_types, status, filters, tags, created, modified
    FROM hookrelay.subscription
    WHERE owner_id = :owner_id
    AND event_types @> :event_types
    `
	} else {
		query = `
    SELECT id, owner_id, target_url, target_method, target_params, target_auth, event_types, status, filters, tags, created, modified
    FROM hookrelay.subscription
    WHERE owner_id = :owner_id
    AND (JSON_CONTAINS(event_types, :event_types) OR JSON_CONTAINS(event_types, '"*"'))
    `
	}

	if query == "" {
		return nil, fmt.Errorf("unsupported database type: %s", config.HRConfig.Database.Type)
	}

	// Convert the event_type into a JSON array for the query
	eventTypeJSON, err := json.Marshal([]string{eventType})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event type: %v", err)
	}

	args := map[string]interface{}{"owner_id": ownerID, "event_types": eventTypeJSON}
	rows, err := r.db.GetDB().NamedQuery(query, args)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No subscriptions found
		}
		slog.Error("DB error", "err", err)
		return nil, fmt.Errorf("failed to query subscriptions: %v", err)
	}
	defer rows.Close()

	var subscriptions []Subscription
	for rows.Next() {
		var s Subscription
		var targetURL sql.NullString
		var targetMethod sql.NullString
		var targetParams, targetAuth []byte
		var eventTypes, filters, tags []byte

		err := rows.Scan(
			&s.ID, &s.OwnerId, &targetURL, &targetMethod, &targetParams, &targetAuth,
			&eventTypes, &s.Status, &filters, &tags, &s.CreatedAt, &s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %v", err)
		}

		// Map target_url to Subscription.Target.HTTPDetails.URL
		if targetURL.Valid {
			_target, err := target.NewHTTPTarget(targetURL.String, targetMethod.String)
			if err != nil {
				return nil, fmt.Errorf("failed to create target: %v", err)
			}
			s.Target = _target
		}

		// Handle JSON fields properly (optional)
		if len(eventTypes) > 0 {
			json.Unmarshal(eventTypes, &s.EventTypes)
		}
		if len(tags) > 0 {
			json.Unmarshal(tags, &s.Tags)
		}

		subscriptions = append(subscriptions, s)
	}
	return subscriptions, nil
}

func (r *SubscriptionModel) HasSubscriptions(subscriptionId string) (bool, error) {
	query := `
    SELECT COUNT(*) FROM hookrelay.subscription WHERE id=:id
    `

	args := map[string]interface{}{"id": subscriptionId}

	var count int
	nstmt, err := r.db.GetDB().PrepareNamed(query)
	if err != nil {
		slog.Error("error in fetching subscription count", "err", err)
		return false, err
	}
	defer nstmt.Close()

	err = nstmt.Get(&count, args)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil // No subscriptions found, no error
		}
		slog.Error("error in fetching subscription count", "err", err)
		return false, err // Database error
	}

	if count > 0 {
		return true, ErrSubscriptionExists
	}

	return false, nil // No subscriptions found
}
