package event

import (
	"encoding/json"
	"time"
)

type (
	EventDeliveryStatus string
)

const (
	// ScheduledEventStatus when an Event has been scheduled for delivery
	ScheduledEventStatus  EventDeliveryStatus = "Scheduled"
	ProcessingEventStatus EventDeliveryStatus = "Processing"
	DiscardedEventStatus  EventDeliveryStatus = "Discarded"
	FailureEventStatus    EventDeliveryStatus = "Failure"
	SuccessEventStatus    EventDeliveryStatus = "Success"
	RetryEventStatus      EventDeliveryStatus = "Retry"
)

type EventDelivery struct {
	ID             int64     `json:"id,omitempty" db:"id"`
	EventType      string    `json:"event_type,omitempty" db:"event_type"`
	Payload        []byte    `json:"payload,omitempty" db:"payload"`
	SubscriptionId string    `json:"subscription_id" db:"subscription_id"`
	StartedAt      time.Time `json:"started_at" db:"started_at"`
	CompletedAt    time.Time `json:"completed_at" db:"completed_at"`
	StatusCode     int       `json:"status_code" db:"status_code"`
	Error          string    `json:"error" db:"error"`
}

func NewEventDelivery(e *Event, subscription_id string, statusCode int, err error) *EventDelivery {
	// Convert map[string]interface{} e.Payload into string
	payloadBytes, cerr := json.Marshal(e.Payload)
	if cerr != nil {
		payloadBytes = []byte("{}")
	}
	delivery := &EventDelivery{
		EventType:      e.EventType,
		Payload:        payloadBytes,
		StartedAt:      e.AcknowledgedAt,
		SubscriptionId: subscription_id,
		CompletedAt:    time.Now(),
		StatusCode:     0,
	}

	delivery.StatusCode = statusCode
	if err != nil {
		delivery.Error = err.Error()
	}
	return delivery
}

func (ed *EventDelivery) IsSuccess() bool {
	return ed.StatusCode >= 200 && ed.StatusCode <= 299
}
