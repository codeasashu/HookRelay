package metrics

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	m    *Metrics
	reg  *prometheus.Registry
	re   sync.Once
	once sync.Once
)

type Metrics struct {
	IsEnabled bool

	// Event Metrics
	IngestTotal          *prometheus.CounterVec
	IngestConsumedTotal  *prometheus.CounterVec
	IngestErrorsTotal    *prometheus.CounterVec
	IngestSuccessTotal   *prometheus.CounterVec
	IngestLatency        *prometheus.HistogramVec
	EventDeliveryLatency *prometheus.HistogramVec
	EventDispatchLatency *prometheus.HistogramVec
	PreFlightLatency     *prometheus.HistogramVec

	// Subscriptions metrics
	TotalSubscriptions *prometheus.GaugeVec
	FanoutSize         *prometheus.HistogramVec

	// Worker Metrics
	WorkerQueueSize    *prometheus.GaugeVec
	WorkerThreadsTotal *prometheus.GaugeVec

	// System metrics
	GoroutineCount *prometheus.GaugeVec
}

func GetDPInstance() *Metrics {
	once.Do(func() {
		m = newMetrics(Reg())
	})
	return m
}

func newMetrics(pr prometheus.Registerer) *Metrics {
	m := InitMetrics()

	if m.IsEnabled {
		pr.MustRegister(
			m.IngestTotal,
			m.IngestConsumedTotal,
			m.IngestErrorsTotal,
			m.IngestSuccessTotal,
			m.IngestLatency,
			m.EventDeliveryLatency,
			m.EventDispatchLatency,
			m.PreFlightLatency,
			m.TotalSubscriptions,
			m.FanoutSize,
			m.GoroutineCount,
			m.WorkerQueueSize,
			m.WorkerThreadsTotal,
		)
	}
	return m
}

const (
	eventLabel            = "event"
	eventTypeLabel        = "event_type"
	targetLabel           = "target"
	listenerLabel         = "listener"
	deliveryLabel         = "deliver"
	ownerLabel            = "owner_id"
	subscriptionTypeLabel = "subscription_type"
	pidLabel              = "pid"
	workerLabel           = "worker_id"
	typeLabel             = "type"
)

func InitMetrics() *Metrics {
	if !config.HRConfig.Metrics.Enabled {
		return &Metrics{
			IsEnabled: false,
		}
	}

	m := &Metrics{
		IsEnabled: true,

		IngestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hookrelay_ingest_total",
				Help: "Total number of events ingested",
			},
			[]string{listenerLabel},
		),
		IngestConsumedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hookrelay_ingest_consumed_total",
				Help: "Total number of events successfully ingested and consumed",
			},
			[]string{eventTypeLabel, listenerLabel},
		),
		IngestErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hookrelay_ingest_errors_total",
				Help: "Total number of errors during event ingestion",
			},
			[]string{eventLabel, eventTypeLabel, listenerLabel},
		),
		IngestSuccessTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hookrelay_ingest_success_total",
				Help: "Total number of successful event ingestion",
			},
			// []string{eventLabel, eventTypeLabel, listenerLabel},
			[]string{eventTypeLabel, listenerLabel},
		),
		IngestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "hookrelay_ingest_latency",
				Help: "Total time (in microsecond) an event spends in HookRelay.",
				// Buckets: prometheus.ExponentialBuckets(100, 2, 10),
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			// []string{eventLabel, eventTypeLabel, listenerLabel},
			[]string{listenerLabel},
		),
		EventDeliveryLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hookrelay_end_to_end_latency",
				Help:    "Total time (in milliseconds) an event spends in HookRelay.",
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
				// Buckets: prometheus.ExponentialBuckets(100, 2, 10),
			},
			// []string{eventLabel, eventTypeLabel, listenerLabel, deliveryLabel},
			[]string{listenerLabel, deliveryLabel},
		),
		EventDispatchLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "hookrelay_event_dispatch_latency",
				Help: "Total time (in microsecond) an event spends after subscription has been found, in HookRelay.",
				// Buckets: prometheus.ExponentialBuckets(100, 2, 10),
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			// []string{eventLabel, eventTypeLabel, listenerLabel},
			[]string{listenerLabel},
		),
		PreFlightLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "hookrelay_event_preflight_latency",
				Help: "Total time (in microsecond) an event spends before being dispatched in HookRelay.",
				// Buckets: prometheus.ExponentialBuckets(100, 2, 10),
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			// []string{eventLabel, eventTypeLabel, listenerLabel},
			[]string{listenerLabel},
		),
		TotalSubscriptions: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hookrelay_total_subscriptions",
				Help: "Total number of active subscriptions in hookrelay",
			},
			[]string{subscriptionTypeLabel},
		),
		FanoutSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hookrelay_fanout_size",
				Help:    "Number of endpoints events are fanned out to",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10),
			},
			// []string{eventLabel, eventTypeLabel},
			[]string{eventTypeLabel},
		),
		WorkerQueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hookrelay_worker_queue_size",
				Help: "Total number of items in the worker queue",
			},
			[]string{workerLabel, typeLabel},
		),
		WorkerThreadsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hookrelay_worker_threads_count",
				Help: "Total number of active workers threads in hookrelay",
			},
			[]string{workerLabel, typeLabel},
		),
		GoroutineCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hookrelay_goroutines_total",
				Help: "Total number of active subscriptions in hookrelay",
			},
			[]string{pidLabel, typeLabel},
		),
	}

	return m
}

func (m *Metrics) RecordEndToEndLatency(ev *event.Event) {
	if !m.IsEnabled {
		return
	}

	d := time.Since(ev.CreatedAt)
	t := float64(d) / float64(time.Millisecond)
	// m.EventDeliveryLatency.With(prometheus.Labels{eventLabel: ev.UID, eventTypeLabel: ev.EventType, listenerLabel: "http", deliveryLabel: "http"}).Observe(t)
	m.EventDeliveryLatency.With(prometheus.Labels{listenerLabel: "http", deliveryLabel: "http"}).Observe(t)
}

func (m *Metrics) RecordPreFlightLatency(ev *event.Event) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(ev.CreatedAt)
	t := float64(d) / float64(time.Microsecond)
	// m.PreFlightLatency.With(prometheus.Labels{eventLabel: ev.UID, eventTypeLabel: ev.EventType, listenerLabel: "http"}).Observe(t)
	m.PreFlightLatency.With(prometheus.Labels{listenerLabel: "http"}).Observe(t)
}

func (m *Metrics) RecordDispatchLatency(ev *event.Event) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(ev.CreatedAt)
	t := float64(d) / float64(time.Microsecond)
	// m.EventDispatchLatency.With(prometheus.Labels{eventLabel: ev.UID, eventTypeLabel: ev.EventType, listenerLabel: "http"}).Observe(t)
	m.EventDispatchLatency.With(prometheus.Labels{listenerLabel: "http"}).Observe(t)
}

func (m *Metrics) RecordIngestLatency(ev *event.Event) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(ev.CreatedAt)
	t := float64(d) / float64(time.Microsecond)
	// m.IngestLatency.With(prometheus.Labels{eventLabel: ev.UID, eventTypeLabel: ev.EventType, listenerLabel: "http"}).Observe(t)
	m.IngestLatency.With(prometheus.Labels{listenerLabel: "http"}).Observe(t)
}

func (m *Metrics) IncrementIngestTotal() {
	if !m.IsEnabled {
		return
	}
	m.IngestTotal.With(prometheus.Labels{listenerLabel: "http"}).Inc()
}

func (m *Metrics) IncrementIngestErrorsTotal(ev *event.Event) {
	if !m.IsEnabled {
		return
	}
	m.IngestErrorsTotal.With(prometheus.Labels{eventLabel: ev.UID, eventTypeLabel: ev.EventType, listenerLabel: "http"}).Inc()
}

func (m *Metrics) IncrementIngestSuccessTotal(ev *event.Event) {
	if !m.IsEnabled {
		return
	}
	// m.IngestSuccessTotal.With(prometheus.Labels{eventLabel: ev.UID, eventTypeLabel: ev.EventType, listenerLabel: "http"}).Inc()
	m.IngestSuccessTotal.With(prometheus.Labels{eventTypeLabel: ev.EventType, listenerLabel: "http"}).Inc()
}

func (m *Metrics) IncrementIngestConsumedTotal(ev *event.Event) {
	if !m.IsEnabled {
		return
	}
	m.IngestConsumedTotal.With(prometheus.Labels{listenerLabel: "http", eventTypeLabel: ev.EventType}).Inc()
}

func (m *Metrics) UpdateTotalSubscriptionCount(count int) {
	if !m.IsEnabled {
		return
	}
	m.TotalSubscriptions.With(prometheus.Labels{subscriptionTypeLabel: "http"}).Set(float64(count))
}

func (m *Metrics) RecordFanout(ev *event.Event, size int) {
	if !m.IsEnabled {
		return
	}
	// m.FanoutSize.With(prometheus.Labels{eventLabel: ev.UID, eventTypeLabel: ev.EventType}).Observe(float64(size))
	m.FanoutSize.With(prometheus.Labels{eventTypeLabel: ev.EventType}).Observe(float64(size))
}

func (m *Metrics) UpdateWorkerQueueSize(workerId string, size int) {
	if !m.IsEnabled {
		return
	}
	m.WorkerQueueSize.With(prometheus.Labels{workerLabel: workerId, typeLabel: "worker"}).Set(float64(size))
}

func (m *Metrics) UpdateWorkerThreadCount(workerId string, count int) {
	if !m.IsEnabled {
		return
	}
	m.WorkerThreadsTotal.With(prometheus.Labels{workerLabel: workerId, typeLabel: "worker"}).Set(float64(count))
}

func (m *Metrics) UpdateGoroutineCount(routineType string) {
	if !m.IsEnabled {
		return
	}
	pid := fmt.Sprintf("%d", os.Getpid())
	count := runtime.NumGoroutine()
	m.GoroutineCount.With(prometheus.Labels{pidLabel: pid, typeLabel: routineType}).Set(float64(count))
}

func Reg() *prometheus.Registry {
	re.Do(func() {
		reg = prometheus.NewPedanticRegistry()
	})

	return reg
}

func (m *Metrics) StartGoroutineMonitor(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Update total goroutines
			m.UpdateGoroutineCount("total")
		case <-ctx.Done():
			return
		}
	}
}
