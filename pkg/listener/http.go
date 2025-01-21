package listener

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/codeasashu/HookRelay/internal/metrics"
)

var m *metrics.Metrics

type HTTPListener struct {
	ListenerChan chan event.Event
	QueueSize    int
	dispatcher   *dispatcher.Dispatcher
	server       *http.Server
}

func (l *HTTPListener) setupWorkers() {
	numWorkers := config.HRConfig.Listener.Http.Workers
	log.Printf("Setting %d HTTP workers\n", numWorkers)
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
	log.Printf("Event ID - %s\n", event.EventType)
	return event, nil
}

func (l *HTTPListener) handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received HTTP Event")
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
	log.Printf("Replied to HTTP Event")
}

func NewHTTPListener(addr string, disp *dispatcher.Dispatcher) *HTTPListener {
	m = metrics.GetDPInstance()
	listener := &HTTPListener{
		server: &http.Server{
			Addr: addr,
		},
		dispatcher:   disp,
		QueueSize:    config.HRConfig.Listener.Http.QueueSize,
		ListenerChan: make(chan event.Event, config.HRConfig.Listener.Http.QueueSize),
	}
	return listener
}

func (l *HTTPListener) StartAndReceive() error {
	l.setupWorkers()
	http.HandleFunc("/event", l.handler)
	log.Printf("Starting HTTP listener on addr - %s\n", l.server.Addr)
	return l.server.ListenAndServe()
}

func (l *HTTPListener) Shutdown(ctx context.Context) {
	if err := l.server.Shutdown(ctx); err != nil {
		log.Fatal("HTTP Server forced to shutdown:", err)
	}
}
