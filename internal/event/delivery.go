package event

import (
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
	EventID        int64     `json:"event_id,omitempty" db:"event_id"`
	OwnerId        string    `json:"owner_id"`
	SubscriptionId string    `json:"subscription_id" db:"subscription_id"`
	StartedAt      time.Time `json:"started_at" db:"started_at"`
	CompletedAt    time.Time `json:"completed_at" db:"completed_at"`
	StatusCode     int       `json:"status_code" db:"status_code"`
	Error          string    `json:"error" db:"error"`
	Latency        float64   `json:"latency" db:"latency"`
}

func NewEventDelivery(e *Event, subscription_id string, statusCode int, err error) *EventDelivery {
	delivery := &EventDelivery{
		EventID:        e.UID,
		OwnerId:        e.OwnerId,
		StartedAt:      e.AcknowledgedAt,
		SubscriptionId: subscription_id,
		CompletedAt:    time.Now(),
		StatusCode:     0,
		Latency:        float64(time.Since(e.CreatedAt)) / float64(time.Microsecond),
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
