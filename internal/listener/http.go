package listener

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/delivery"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/subscription"
	"github.com/codeasashu/HookRelay/internal/wal"
	"github.com/gin-gonic/gin"
)

type HTTPListener struct {
	router       *gin.Engine
	numWorkers   int
	listenerChan chan *event.Event
	stopChan     chan struct{}
	QueueSize    int
	metrics      *metrics.Metrics
	delivery     delivery.Delivery
	subscription *subscription.Subscription
	wal          wal.AbstractWAL
}

func NewHTTPListener(f *app.HookRelayApp, dl delivery.Delivery, subscriptionApp *subscription.Subscription) *HTTPListener {
	listener := &HTTPListener{
		QueueSize:    f.Cfg.Listener.Http.QueueSize,
		numWorkers:   f.Cfg.Listener.Http.Workers,
		subscription: subscriptionApp,
		listenerChan: make(chan *event.Event, f.Cfg.Listener.Http.QueueSize),
		metrics:      f.Metrics,
		delivery:     dl,
		wal:          f.WAL,
		stopChan:     make(chan struct{}),
		router:       f.Router,
	}
	slog.Info("Setting HTTP workers", "num", listener.numWorkers)
	for i := 0; i < listener.numWorkers; i++ {
		go listener.setupWorker()
	}
	return listener
}

func (l *HTTPListener) InitApiRoutes() {
	{
		v1 := l.router.Group("/event")
		v1.POST("", func(c *gin.Context) {
			slog.Info("Received HTTP Event")
			// i := r.URL.Query().Get("id")
			e, err := l.transformEvent(c.Request)
			l.metrics.RecordIngestLatency(e.CreatedAt)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
				return
			}
			l.listenerChan <- e
			c.JSON(http.StatusOK, gin.H{"status": "success", "message": "event received"})
			slog.Info("Replied to HTTP Event")
		})
	}
}

func (l *HTTPListener) setupWorker() {
	for {
		select {
		case event := <-l.listenerChan:
			if event == nil {
				return
			}
			slog.Info("Received event from listener channel", "event", event)
			l.wal.Log(event.LogEvent)
			subscriptions, err := l.subscription.FindSubscriptionsByEventTypeAndOwner(event.EventType, event.OwnerId, true)
			if err != nil {
				slog.Error("error fetching subscriptions", "err", err)
				continue
			}

			slog.Info("fetched subscriptions", "event_id", event.UID, "fanout", len(subscriptions), "event_type", event.EventType, "owner_id", event.OwnerId)
			l.metrics.RecordPreFlightLatency(event)
			if len(subscriptions) == 0 {
				continue
			}
			for _, sub := range subscriptions {
				deliver := delivery.NewEventDelivery(event, &sub)
				err := l.delivery.Schedule(deliver)
				// err := l.workerPool.Schedule(deliver)
				if err != nil {
					slog.Error("error scheduling job", "delivery_id", deliver.ID, "err", err)
					continue
				}
			}
		case <-l.stopChan:
			return
			// default:
			// 	return
		}
	}
}

func (l *HTTPListener) transformEvent(req *http.Request) (*event.Event, error) {
	decoder := json.NewDecoder(req.Body)
	event := event.New()
	err := decoder.Decode(event)
	if err != nil {
		return nil, errors.New("failed to decode event payload. invalid json")
	}
	l.metrics.IncrementIngestTotal()
	event.Ack()
	slog.Info("Acknowledge evnet", "id", event.UID, "type", event.EventType)
	return event, nil
}

func (l *HTTPListener) Shutdown(ctx context.Context) {
	slog.Info("shutting down HTTP listener...")
	close(l.listenerChan)
	close(l.stopChan)
}
