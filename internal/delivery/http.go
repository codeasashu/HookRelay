package delivery

import (
	"encoding/json"
	"time"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/subscription"
	"github.com/codeasashu/HookRelay/internal/wal"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
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

type HTTPDelivery struct {
	router  *gin.Engine
	metrics *metrics.Metrics
	wp      *worker.WorkerPool
	wl      wal.AbstractWAL
}

func NewHTTPDelivery(a *app.HookRelayApp, wp *worker.WorkerPool) (*HTTPDelivery, error) {
	return &HTTPDelivery{router: a.Router, metrics: a.Metrics, wp: wp, wl: a.WAL}, nil
}

func (d *HTTPDelivery) Schedule(e *event.Event, s *subscription.Subscriber) error {
	payloadBytes, cerr := json.Marshal(e.Payload)
	if cerr != nil {
		payloadBytes = []byte("{}")
	}
	ed := &EventDelivery{
		EventType:      e.EventType,
		Payload:        payloadBytes,
		StartAt:        e.CreatedAt,
		OwnerId:        e.OwnerId,
		SubscriptionId: s.ID,
		CompleteAt:     time.Now(), // @TODO: Use zero time/nil value to indicate incomplete event
		StatusCode:     0,
		TraceId:        e.TraceId,

		Subscriber: s,
		MaxRetries: 1,
	}
	return d.wp.Schedule(ed, false)
}
