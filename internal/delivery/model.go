package delivery

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
)

type EventDeliveryQueryParams struct {
	EventType string
	OwnerId   string
	Limit     uint16
	Offset    uint16
	CreatedAt *time.Time
}

func cursorToTime(ms int64) *time.Time {
	seconds := ms / 1e6
	nanoseconds := (ms % 1e6) * 1e3
	cursorVal := time.Unix(seconds, nanoseconds).UTC()
	return &cursorVal
}

func timeToCursor(val *time.Time) int64 {
	return val.UnixMicro()
}

func GetDeliveriesByOwner(
	db *sqlx.DB,
	ownerId string,
	eventType *string,
	createdAfter *time.Time,
	createdBefore *time.Time,
	cursor int64, // Cursor-based pagination (unix nano)
	limit uint16,
) (uint64, []*EventDelivery, int64, error) {
	var totalCount uint64
	// Count Query (Remains the same)
	countQuery := `
        SELECT COUNT(*) FROM event_delivery ed WHERE ed.owner_id = ?`

	// Cursor-Based Query (Instead of OFFSET)
	query := `
        SELECT ed.id, ed.event_type, ed.payload, ed.owner_id, ed.subscription_id, 
               ed.status_code, ed.error, ed.start_at, ed.complete_at
        FROM event_delivery ed WHERE ed.owner_id = ?`

	countArgs := []interface{}{ownerId}
	args := []interface{}{ownerId}

	if *eventType != "" {
		countQuery += " AND ed.event_type = ?"
		query += " AND ed.event_type = ?"
		countArgs = append(countArgs, *eventType)
		args = append(args, *eventType)
	}

	if createdAfter != nil {
		countQuery += " AND ed.start_at >= ?"
		query += " AND ed.start_at >= ?"
		countArgs = append(countArgs, *createdAfter)
		args = append(args, *createdAfter)
	}

	if createdBefore != nil {
		countQuery += " AND ed.start_at <= ?"
		query += " AND ed.start_at <= ?"
		countArgs = append(countArgs, *createdBefore)
		args = append(args, *createdBefore)
	}

	if err := db.QueryRow(countQuery, countArgs...).Scan(&totalCount); err != nil {
		return 0, nil, 0, fmt.Errorf("failed to get total count: %v", err)
	}

	if cursor != 0 {
		query += " AND ed.start_at < ?"
		cursorVal := cursorToTime(cursor)
		args = append(args, cursorVal)
	}

	query += " ORDER BY ed.start_at DESC, ed.id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, 0, fmt.Errorf("failed to query deliveries: %v", err)
	}
	defer rows.Close()

	batch := make([]*EventDelivery, 0)
	var nextCursor int64
	for rows.Next() {
		var e EventDelivery
		if err := rows.Scan(
			&e.ID, &e.EventType, &e.Payload, &e.OwnerId, &e.SubscriptionId,
			&e.StatusCode, &e.Error, &e.StartAt, &e.CompleteAt,
		); err != nil {
			slog.Error("failed to scan event delivery", slog.Any("error", err))
			continue
		}
		batch = append(batch, &e)
		startAt := &e.StartAt // Store the last seen timestamp for next cursor
		slog.Info("storing cursor for", "startAt", startAt, "cursor", startAt.UnixMicro())
		nextCursor = startAt.UnixMicro()
	}

	slog.Info("next cursor", "cursor", nextCursor, "addr", &nextCursor)
	if err := rows.Err(); err != nil {
		return 0, nil, 0, err
	}

	// Reset cursor if no more results are remaining
	if len(batch) >= int(totalCount) {
		nextCursor = 0
	}

	return totalCount, batch, nextCursor, nil
}
