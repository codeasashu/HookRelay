package listener

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/gin-gonic/gin"
)

var m *metrics.Metrics

type HTTPListener struct {
	app          *cli.App
	ListenerChan chan event.Event
	QueueSize    int
	dispatcher   *dispatcher.Dispatcher
}

func (l *HTTPListener) setupWorkers() {
	numWorkers := config.HRConfig.Listener.Http.Workers
	slog.Info("Setting HTTP workers", "num", numWorkers)
	for i := 0; i < numWorkers; i++ {
		go l.dispatcher.ListenForEvents(l.ListenerChan)
	}
}

func (l *HTTPListener) transformEvent(req *http.Request) (*event.Event, error) {
	decoder := json.NewDecoder(req.Body)
	event := event.New()
	err := decoder.Decode(event)
	if err != nil {
		return nil, errors.New("failed to decode event payload. invalid json")
	}
	m.IncrementIngestTotal()
	event.Ack()
	event.Save(l.app)
	slog.Info("Acknowledge evnet", "id", event.UID, "type", event.EventType)
	return event, nil
}

func (l *HTTPListener) handler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received HTTP Event")
	// i := r.URL.Query().Get("id")
	e, err := l.transformEvent(r)
	m.RecordIngestLatency(e)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	l.ListenerChan <- *e
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Event received"))
	slog.Info("Replied to HTTP Event")
}

func (l *HTTPListener) createSubscriptionHandler() gin.HandlerFunc {
	l.setupWorkers()
	return func(c *gin.Context) {
		slog.Info("Received HTTP Event")
		// i := r.URL.Query().Get("id")
		e, err := l.transformEvent(c.Request)
		m.RecordIngestLatency(e)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
			return
		}
		l.ListenerChan <- *e
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "event received"})
		slog.Info("Replied to HTTP Event")
	}
}

func NewHTTPListener(disp *dispatcher.Dispatcher) *HTTPListener {
	m = metrics.GetDPInstance()
	app := cli.GetAppInstance()
	listener := &HTTPListener{
		app:          app,
		dispatcher:   disp,
		QueueSize:    config.HRConfig.Listener.Http.QueueSize,
		ListenerChan: make(chan event.Event, config.HRConfig.Listener.Http.QueueSize),
	}
	return listener
}

func (l *HTTPListener) AddRoutes(server *api.ApiServer) {
	{
		v1 := server.Router.Group("/event")
		v1.POST("", l.createSubscriptionHandler())
	}
}

func (l *HTTPListener) Shutdown(ctx context.Context) {
	slog.Info("shutting down HTTP listener...")
	close(l.ListenerChan)
}
