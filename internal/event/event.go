package event

import (
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
