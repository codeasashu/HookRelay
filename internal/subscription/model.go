package subscription

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/codeasashu/HookRelay/internal/target"
)

type SubscriptionStatus int

const (
	SubscriptionDeleted SubscriptionStatus = 0
	SubscriptionActive  SubscriptionStatus = 1
)

var (
	ErrSubscriptionNotCreated = errors.New("subscription could not be created")
	ErrSubscriptionNotUpdated = errors.New("subscription could not be updated")
	ErrSubscriptionNotFound   = errors.New("subscription could not be found")
	ErrSubscriptionExists     = errors.New("subscription already exists")
	ErrLegacySubscription     = errors.New("creating subscription in legacy mode is not allowed")
)

func (r *Subscription) Validate(s *Subscriber) error {
	if err := target.ValidateTarget(*s.Target); err != nil {
		slog.Error("error validating subscription target", "err", err, "target", *s.Target)
		return err
	}

	// Existing Subscription does not exist
	found, err := r.HasSubscriptions(s.ID)
	if err != nil {
		slog.Error("error validating existing subscriptions", "err", err, "id", s.ID, "owner_id", s.OwnerId)
		return err
	}
	if found {
		slog.Error("error validating existing subscriptions", "err", err, "id", s.ID, "owner_id", s.OwnerId)
		return ErrSubscriptionExists
	}
	return nil
}

func (r *Subscription) HasSubscriptions(subscriptionId string) (bool, error) {
	query := `
    SELECT COUNT(*) FROM hookrelay.subscription WHERE id=:id AND status = 1
    `

	args := map[string]any{"id": subscriptionId}

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
		return true, nil
	}

	return false, nil // No subscriptions found
}

type DBSubscription struct {
	ID            string         `db:"id"`
	OwnerID       string         `db:"owner_id"`
	TargetMethod  string         `db:"target_method"`
	TargetURL     string         `db:"target_url"`
	TargetParams  sql.NullString `db:"target_params"` // Example: {"query": {"foo":"bar"}} or null
	TargetAuthRaw sql.NullString `db:"target_auth"`   // JSON string for auth, e.g., {"username":"u","password":"p"} or {} or null
	EventTypesRaw sql.NullString `db:"event_types"`   // JSON string array, e.g., ["event1", "*"] or null
	Status        int            `db:"status"`
	FiltersRaw    sql.NullString `db:"filters"` // JSON string (object or array), e.g., [] or {"field":"value"} or null
	TagsRaw       sql.NullString `db:"tags"`    // JSON string array, e.g., ["tag1"] or null
	Created       time.Time      `db:"created"`
	Modified      time.Time      `db:"modified"`
}

func (dbSub *DBSubscription) ToSubscriber() (Subscriber, error) {
	var sub Subscriber
	sub.ID = dbSub.ID
	sub.OwnerId = dbSub.OwnerID
	sub.Status = dbSub.Status
	sub.CreatedAt = dbSub.Created
	sub.ModifiedAt = dbSub.Modified

	// --- Populate Target ---
	httpDetails := &target.HTTPDetails{
		URL:    dbSub.TargetURL,
		Method: target.HTTPMethod(dbSub.TargetMethod), // Cast to your HTTPMethod type
		// Headers: Populated from target_params if applicable, or managed elsewhere.
		// The current flat schema doesn't have a dedicated headers column.
		// Initialize if needed: Headers: make(map[string]string),
	}

	// Handle TargetAuth (BasicAuth)
	if dbSub.TargetAuthRaw.Valid && dbSub.TargetAuthRaw.String != "" {
		var basicAuth target.HTTPBasicAuth
		// An empty JSON object "{}" will unmarshal to an empty basicAuth struct, which is fine (no auth).
		if err := json.Unmarshal([]byte(dbSub.TargetAuthRaw.String), &basicAuth); err != nil {
			slog.Error("Failed to unmarshal target_auth", "id", dbSub.ID, "raw_auth", dbSub.TargetAuthRaw.String, "error", err)
			return sub, fmt.Errorf("invalid target_auth JSON for subscription %s: %w", dbSub.ID, err)
		}
		httpDetails.BasicAuth = basicAuth // Assign struct value
	}

	sub.Target = &target.Target{
		Type:        target.TargetHTTP, // Assuming 'http' as type based on flat columns
		HTTPDetails: httpDetails,
	}

	// --- Populate EventTypes ---
	if dbSub.EventTypesRaw.Valid && dbSub.EventTypesRaw.String != "" {
		if err := json.Unmarshal([]byte(dbSub.EventTypesRaw.String), &sub.EventTypes); err != nil {
			slog.Error("Failed to unmarshal event_types", "id", dbSub.ID, "raw_events", dbSub.EventTypesRaw.String, "error", err)
			return sub, fmt.Errorf("invalid event_types JSON for subscription %s: %w", dbSub.ID, err)
		}
	} else {
		sub.EventTypes = []string{} // Default to empty slice if DB value is NULL or empty string
	}

	// --- Populate Filters ---
	if dbSub.FiltersRaw.Valid && dbSub.FiltersRaw.String != "" {
		sub.Filters = json.RawMessage(dbSub.FiltersRaw.String)
	} else {
		// Default for json.RawMessage if null/empty in DB.
		// Set to valid empty JSON array "[]" or object "{}" if required, or leave as nil (RawMessage zero value).
		// The sample `filters: []` suggests it might often be an empty array.
		// If FiltersRaw.String is "[]", sub.Filters will be json.RawMessage("[]")
		// If FiltersRaw is SQL NULL, sub.Filters will be nil.
		// To ensure it's always valid non-nil RawMessage, you could default to `json.RawMessage("null")` or `json.RawMessage("{}")`
		sub.Filters = json.RawMessage("[]") // Default to empty JSON array if null/empty, adjust if object is preferred
		if dbSub.FiltersRaw.Valid && dbSub.FiltersRaw.String != "" {
			sub.Filters = json.RawMessage(dbSub.FiltersRaw.String)
		}
	}

	// --- Populate Tags ---
	if dbSub.TagsRaw.Valid && dbSub.TagsRaw.String != "" {
		if err := json.Unmarshal([]byte(dbSub.TagsRaw.String), &sub.Tags); err != nil {
			slog.Error("Failed to unmarshal tags", "id", dbSub.ID, "raw_tags", dbSub.TagsRaw.String, "error", err)
			return sub, fmt.Errorf("invalid tags JSON for subscription %s: %w", dbSub.ID, err)
		}
	} else {
		sub.Tags = []string{} // Default to empty slice
	}

	// Note: dbSub.TargetParams is not explicitly mapped here.
	// If TargetParams should populate HTTPDetails.Headers or another field, add that logic.
	// For example, if TargetParams contains query parameters to be appended to the URL,
	// or if it's a JSON object representing headers.

	return sub, nil
}

func (r *Subscription) DeleteSubscriber(id string) (bool, error) {
	found, err := r.HasSubscriptions(id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("subscription %s not found", id)
	}

	_, err = r.db.GetDB().NamedExec(`DELETE FROM hookrelay.subscription WHERE id = :id`, map[string]any{"id": id})
	if err != nil {
		return false, fmt.Errorf("failed to delete subscription %s: %w", id, err)
	}
	return true, nil
}

func (r *Subscription) GetSubscribers(ownerId, eventType string, limit, offset int) ([]Subscriber, bool, error) {
	query := `
    SELECT id, owner_id, target_url, target_method, target_params, target_auth, event_types, status, filters, tags, created, modified
    FROM hookrelay.subscription
    WHERE status = 1`
	if eventType != "" {
		query = fmt.Sprintf("%s AND (JSON_CONTAINS(event_types, :event_types) OR JSON_CONTAINS(event_types, '\"*\"'))", query)
	}
	if ownerId != "" {
		query = fmt.Sprintf("%s AND owner_id = :owner_id", query)
	}
	query = fmt.Sprintf("%s ORDER BY modified DESC, id DESC LIMIT :limit_val OFFSET :offset_val", query)
	// Convert the event_type into a JSON array for the query
	eventTypeJSON, err := json.Marshal([]string{eventType})
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal event type: %v", err)
	}

	effectiveLimit := limit + 1
	args := map[string]any{
		"limit_val":   effectiveLimit,
		"offset_val":  offset,
		"owner_id":    ownerId,
		"event_types": eventTypeJSON,
	}

	rows, err := r.db.GetDB().NamedQuery(query, args) // Ensure r.db provides *sqlx.DB
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []Subscriber{}, false, nil
		}
		slog.Error("DB error querying subscriptions", "error", err)
		return nil, false, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []Subscriber
	for rows.Next() {
		var s Subscriber
		var targetURL sql.NullString
		var targetMethod sql.NullString
		var targetParamsBytes, targetAuthBytes, eventTypes, filters, tags []byte

		err := rows.Scan(
			&s.ID, &s.OwnerId, &targetURL, &targetMethod, &targetParamsBytes, &targetAuthBytes,
			&eventTypes, &s.Status, &filters, &tags, &s.CreatedAt, &s.CreatedAt,
		)
		if err != nil {
			return nil, false, fmt.Errorf("failed to scan subscription: %v", err)
		}

		var targetParams map[string]string
		if len(targetParamsBytes) > 0 {
			if err := json.Unmarshal(targetParamsBytes, &targetParams); err != nil {
				return nil, false, fmt.Errorf("failed to unmarshal target_params: %v", err)
			}
		}

		// Unmarshal target_auth (assuming it's also JSON)
		var targetAuth target.HTTPBasicAuth
		if len(targetAuthBytes) > 0 {
			if err := json.Unmarshal(targetAuthBytes, &targetAuth); err != nil {
				return nil, false, fmt.Errorf("failed to unmarshal target_auth: %v", err)
			}
		}

		// Map target_url to Subscription.Target.HTTPDetails.URL
		if targetURL.Valid {
			_target, err := target.NewHTTPTarget(
				targetURL.String,
				targetMethod.String,
				targetParams,
				targetAuth,
			)
			if err != nil {
				return nil, false, fmt.Errorf("failed to create target: %v", err)
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

	var hasMore bool
	processedCount := len(subscriptions)
	if processedCount > limit {
		hasMore = true
	} else {
		hasMore = false
	}
	return subscriptions, hasMore, nil
}

func (r *Subscription) CreateSubscriber(s *Subscriber) error {
	if err := r.Validate(s); err != nil {
		slog.Error("error validating subscription", "err", err)
		return err
	}

	query := `
    INSERT INTO hookrelay.subscription (id, owner_id, target_url, target_method, target_params, target_auth, event_types, status, filters, tags, created, modified)
    VALUES (:id, :owner_id, :target_url, :target_method, :target_params, :target_auth, :event_types, :status, :filters, :tags, :created, :modified)
    `
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

	headers, err := json.Marshal(s.Target.HTTPDetails.Headers)
	if err != nil {
		slog.Error("failed to marshal subscription headers", "err", err)
		return err
	}

	auth, err := json.Marshal(s.Target.HTTPDetails.BasicAuth)
	if err != nil {
		slog.Error("failed to marshal subscription auth", "err", err)
		return err
	}

	args := map[string]any{
		"id":            s.ID,
		"owner_id":      s.OwnerId,
		"target_url":    s.Target.HTTPDetails.URL,
		"target_method": s.Target.HTTPDetails.Method,
		"target_params": headers,
		"target_auth":   auth,
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

func (r *Subscription) UpdateSubscriber(id string, s *Subscriber) (bool, error) {
	found, err := r.HasSubscriptions(id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("subscription %s not found", id)
	}

	query := `
    UPDATE hookrelay.subscription SET target_url = :target_url, target_method = :target_method, target_params = :target_params, target_auth = :target_auth, event_types = :event_types, status = :status, filters = :filters, tags = :tags WHERE id = :id
    `
	eventTypes, err := json.Marshal(s.EventTypes)
	if err != nil {
		slog.Error("failed to marshal subscription event types", "err", err)
		return false, err
	}

	tags, err := json.Marshal(s.Tags)
	if err != nil {
		slog.Error("failed to marshal subscription tags", "err", err)
		return false, err
	}

	headers, err := json.Marshal(s.Target.HTTPDetails.Headers)
	if err != nil {
		slog.Error("failed to marshal subscription headers", "err", err)
		return false, err
	}

	auth, err := json.Marshal(s.Target.HTTPDetails.BasicAuth)
	if err != nil {
		slog.Error("failed to marshal subscription auth", "err", err)
		return false, err
	}

	args := map[string]any{
		"id":            id,
		"target_url":    s.Target.HTTPDetails.URL,
		"target_method": s.Target.HTTPDetails.Method,
		"target_params": headers,
		"target_auth":   auth,
		"event_types":   eventTypes,
		"status":        int(SubscriptionActive),
		"filters":       "[]",
		"tags":          tags,
	}

	slog.Info("updating subscription", "id", s.ID)

	result, err := r.db.GetDB().NamedExec(query, args)
	if err != nil {
		slog.Error("DB error updating subscription", "err", err)
		return false, ErrSubscriptionNotUpdated
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected < 1 {
		return false, nil
	}

	return true, nil
}

func (r *Subscription) FindSubscriptionsByEventTypeAndOwner(eventType, ownerID string) ([]Subscriber, error) {
	query := `
    SELECT id, owner_id, target_url, target_method, target_params, target_auth, event_types, status, filters, tags, created, modified
    FROM hookrelay.subscription
    WHERE owner_id = :owner_id
    AND (JSON_CONTAINS(event_types, :event_types) OR JSON_CONTAINS(event_types, '"*"'))
	AND status = 1
    `
	// Convert the event_type into a JSON array for the query
	eventTypeJSON, err := json.Marshal([]string{eventType})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event type: %v", err)
	}

	args := map[string]any{"owner_id": ownerID, "event_types": eventTypeJSON}
	rows, err := r.db.GetDB().NamedQuery(query, args)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No subscriptions found
		}
		slog.Error("DB error", "err", err)
		return nil, fmt.Errorf("failed to query subscriptions: %v", err)
	}
	defer rows.Close()

	var subscriptions []Subscriber
	for rows.Next() {
		var s Subscriber
		var targetURL sql.NullString
		var targetMethod sql.NullString
		var targetParamsBytes, targetAuthBytes, eventTypes, filters, tags []byte

		err := rows.Scan(
			&s.ID, &s.OwnerId, &targetURL, &targetMethod, &targetParamsBytes, &targetAuthBytes,
			&eventTypes, &s.Status, &filters, &tags, &s.CreatedAt, &s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %v", err)
		}

		var targetParams map[string]string
		if len(targetParamsBytes) > 0 {
			if err := json.Unmarshal(targetParamsBytes, &targetParams); err != nil {
				return nil, fmt.Errorf("failed to unmarshal target_params: %v", err)
			}
		}

		// Unmarshal target_auth (assuming it's also JSON)
		var targetAuth target.HTTPBasicAuth
		if len(targetAuthBytes) > 0 {
			if err := json.Unmarshal(targetAuthBytes, &targetAuth); err != nil {
				return nil, fmt.Errorf("failed to unmarshal target_auth: %v", err)
			}
		}

		// Map target_url to Subscription.Target.HTTPDetails.URL
		if targetURL.Valid {
			_target, err := target.NewHTTPTarget(
				targetURL.String,
				targetMethod.String,
				targetParams,
				targetAuth,
			)
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
