package event

import (
	"time"

	"github.com/oklog/ulid/v2"
)

type Event struct {
	UID            string      `json:"uid"`
	Payload        interface{} `json:"payload"`
	OwnerId        string      `json:"owner_id"`
	EventType      string      `json:"event_type"`
	CreatedAt      time.Time
	AcknowledgedAt time.Time
	CompletedAt    time.Time
}

func New() *Event {
	e := &Event{
		UID:       ulid.Make().String(),
		CreatedAt: time.Now(),
	}
	return e
}

func (e *Event) Ack() {
	e.AcknowledgedAt = time.Now()
}
