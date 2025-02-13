package event

import (
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

func GetDeliveriesByOwner(db *sqlx.DB, ownerId string, limit uint16, offset uint16) ([]*EventDelivery, error) {
	rows, err := db.Query(`
        SELECT event_type, payload, subscription_id, status_code, error, created_at FROM event_delivery ed
        INNER JOIN SUBSCRIPTION s ON s.id = ed.subscription_id WHERE s.owner_id = ? LIMIT ? OFFSET ?
        `, ownerId, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query deliveries: %v", err)
	}

	batch := make([]*EventDelivery, 0)
	for rows.Next() {
		var e EventDelivery
		if err := rows.Scan(&e.EventType, &e.Payload, &e.SubscriptionId, &e.StatusCode, &e.Error, &e.CompletedAt); err != nil {
			slog.Error("failed to scan WAL file", slog.Any("error", err))
			continue
		}

		batch = append(batch, &e)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}

	rows.Close()
	return batch, nil
}
