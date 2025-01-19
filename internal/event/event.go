package event

import "time"

type Event struct {
	Payload   interface{} `json:"payload"`
	OwnerId   string      `json:"owner_id"`
	EventType string      `json:"event_type"`

	LatencyTimestamps map[string]time.Time
}

func New() *Event {
	e := &Event{
		LatencyTimestamps: make(map[string]time.Time),
	}
	return e
}

func (e *Event) AddLatencyTimestamp(key string) {
	e.LatencyTimestamps[key] = time.Now()
}
