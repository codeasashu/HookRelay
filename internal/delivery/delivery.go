package delivery

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/subscription"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/prometheus/client_golang/prometheus"
)

type Delivery interface {
	Schedule(job *EventDelivery) error
}

type EventDelivery struct {
	ID             int64     `json:"id,omitempty" db:"id"`
	EventType      string    `json:"event_type,omitempty" db:"event_type"`
	Payload        []byte    `json:"payload,omitempty" db:"payload"`
	OwnerId        string    `json:"owner_id" db:"owner_id"`
	SubscriptionId string    `json:"subscription_id" db:"subscription_id"`
	StartAt        time.Time `json:"start_at" db:"start_at"`
	CompleteAt     time.Time `json:"complete_at" db:"complete_at"`
	StatusCode     int       `json:"status_code" db:"status_code"`
	Error          string    `json:"error" db:"error"`

	Subscriber      *subscription.Subscriber `json:"subscriber"`
	MaxRetries      uint8                    `json:"max_retries"`
	TotalDeliveries atomic.Int32             `json:"total_deliveries"`
}

func EventDeliveryUnmarshaler() func([]byte) (worker.Task, error) {
	return func(data []byte) (worker.Task, error) {
		var ed EventDelivery
		err := json.Unmarshal(data, &ed)
		return &ed, err
	}
}

func EventDeliveryMarshaler() func(worker.Task) ([]byte, error) {
	return func(task worker.Task) ([]byte, error) {
		ed := task.(*EventDelivery)
		if ed.Subscriber == nil {
			slog.Error("invalid subscription 11")
		}
		if ed.Subscriber.Target == nil {
			slog.Error("invalid target")
		}
		return json.Marshal(ed)
	}
}

func (ed *EventDelivery) GetID() string {
	return ed.Subscriber.ID
}

func (ed *EventDelivery) GetType() string {
	return ed.EventType
}

func (ed *EventDelivery) NumDeliveries() int {
	return int(ed.TotalDeliveries.Load())
}

func (ed *EventDelivery) IncDeliveries() {
	ed.TotalDeliveries.Add(1)
}

func (ed *EventDelivery) Execute(wrkr worker.Worker) error {
	targetStartTime := time.Now()
	m := wrkr.GetMetricsHandler()
	wrkrType := wrkr.GetType()
	if m != nil && m.IsEnabled {
		m.RecordPreFlightLatency("http", &ed.StartAt)
	}
	if ed.Subscriber == nil || ed.Subscriber.Target == nil {
		return fmt.Errorf("invalid event subscription")
	}
	statusCode, err := ed.Subscriber.Target.ProcessTarget(ed.Payload)
	if err != nil {
		slog.Warn("error executing event delivery", "id", ed.GetID(), "target", ed.Subscriber.Target.HTTPDetails.URL, "delivery", ed.ID, "err", err)
		ed.Error = err.Error()
	} else {
		slog.Warn("event delivery complete", "statusCode", statusCode, "delivery", ed.ID)
	}
	ed.StatusCode = statusCode
	ed.CompleteAt = time.Now()
	if m != nil && m.IsEnabled {
		m.IncTotalDeliveries(ed.EventType, string(wrkrType), strconv.Itoa(statusCode))

		d := time.Since(targetStartTime)
		t := float64(d) / float64(time.Millisecond)
		method := string(ed.Subscriber.Target.HTTPDetails.Method)
		m.TargetLatency.With(prometheus.Labels{
			metrics.OwnerLabel:        ed.OwnerId,
			metrics.TargetUrlLabel:    ed.Subscriber.Target.HTTPDetails.URL,
			metrics.TargetMethodLabel: method,
		}).Observe(t)

		d = time.Since(ed.StartAt)
		t = float64(d) / float64(time.Millisecond)
		m.DeliveryLatency.With(prometheus.Labels{
			metrics.OwnerLabel:          ed.OwnerId,
			metrics.EventTypeLabel:      ed.EventType,
			metrics.WorkerLabel:         string(wrkrType),
			metrics.DeliveryStatusLabel: strconv.Itoa(statusCode),
		}).Observe(t)
	}

	return err
}

func (ed *EventDelivery) Retries() int {
	return int(ed.MaxRetries)
}

func SaveDeliveries(f *app.HookRelayApp) func(deliveries []worker.Task) error {
	if f.WAL != nil {
		return func(deliveries []worker.Task) error {
			return LogBatchEventDelivery(f.WAL, deliveries)
		}
	} else if f.DeliveryDb != nil {
		return func(deliveries []worker.Task) error {
			startTime := time.Now()
			err := SaveBatchEventDelivery(f.DeliveryDb, deliveries)
			f.Metrics.RecordDeliveryDbLatency("direct", &startTime)
			return err
		}
	}
	return nil
}

func NewEventDelivery(e *event.Event, subscriber *subscription.Subscriber) *EventDelivery {
	// Convert map[string]interface{} e.Payload into string
	payloadBytes, cerr := json.Marshal(e.Payload)
	if cerr != nil {
		payloadBytes = []byte("{}")
	}
	delivery := &EventDelivery{
		EventType:      e.EventType,
		Payload:        payloadBytes,
		StartAt:        e.CreatedAt,
		OwnerId:        e.OwnerId,
		SubscriptionId: subscriber.ID,
		CompleteAt:     time.Now(),
		StatusCode:     0,

		Subscriber: subscriber,
		MaxRetries: 1,
	}
	return delivery
}

func (ed *EventDelivery) IsSuccess() bool {
	return ed.StatusCode >= 200 && ed.StatusCode <= 299
}
