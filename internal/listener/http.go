package listener

import (
	"context"
	"log/slog"
	"net/http"
	"time"

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
			e, err := l.transformEvent(c.Request)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
				return
			}
			l.metrics.IncrementIngestTotal("http")
			l.metrics.RecordIngestLatency("http", e.CreatedAt)
			l.listenerChan <- e
			c.JSON(http.StatusOK, gin.H{"status": "success", "message": "event received"})
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
			slog.Info("listener channel picked event", "trace_id", event.TraceId)
			if l.wal != nil {
				l.wal.Log(event.LogEvent)
			}
			startTime := time.Now()
			subscriptions, err := l.subscription.FindSubscriptionsByEventTypeAndOwner(event.EventType, event.OwnerId)
			l.metrics.RecordSubscriberDbLatency(event.OwnerId, event.EventType, &startTime)
			if err != nil {
				slog.Error("error fetching subscriptions", "trace_id", event.TraceId, "err", err)
				continue
			}
			l.metrics.UpdateTotalDeliverables(event.EventType, len(subscriptions))
			if len(subscriptions) == 0 {
				slog.Warn("no event subscriptions found", "event_type", event.EventType, "trace_id", event.TraceId, "fanout", len(subscriptions), "owner_id", event.OwnerId)
				continue
			}
			slog.Info("fetched event subscriptions", "event_type", event.EventType, "trace_id", event.TraceId, "fanout", len(subscriptions), "owner_id", event.OwnerId)
			for _, sub := range subscriptions {
				err := l.delivery.Schedule(event, &sub)
				if err != nil {
					slog.Error("error scheduling event", "trace_id", event.TraceId, "subscription_id", sub.ID, "err", err)
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
	// Get `X-MYOP-TRACE-ID` header from req
	event, err := event.NewFromHTTP(req)
	if err != nil {
		return nil, err
	}
	event.Ack()
	return event, nil
}

func (l *HTTPListener) Shutdown(ctx context.Context) {
	slog.Info("shutting down HTTP listener...")
	close(l.listenerChan)
	close(l.stopChan)
}
