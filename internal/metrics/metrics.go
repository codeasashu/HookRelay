package metrics

import (
	"log/slog"
	"sync"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/hibiken/asynq"
	qmetrics "github.com/hibiken/asynq/x/metrics"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type key string

const (
	MetricsContextKey key = "_minstance" // To get isntance by context key, in workers
)

var (
	reg *prometheus.Registry
	re  sync.Once
)

type Metrics struct {
	IsEnabled bool
	Registery *prometheus.Registry
	router    *gin.Engine

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
}

func NewMetrics(cfg *config.MetricsConfig) *Metrics {
	m := InitMetrics(cfg)

	if m.IsEnabled {
		m.Registery.MustRegister(
			// Add the standard process and go metrics to the registry
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			collectors.NewGoCollector(),
			m.IngestTotal,
			m.IngestLatency,
			m.TotalSubscriptions,
			m.FanoutSize,
			m.PreFlightLatency,

			// Local worker metrics
			m.IngestConsumedTotal,
			m.IngestErrorsTotal,
			m.IngestSuccessTotal,
			m.EventDispatchLatency,
			m.EventDeliveryLatency,
			m.WorkerQueueSize,
			m.WorkerThreadsTotal,
		)
	}
	return m
}

func (m *Metrics) RegisterWorkerMetrics(cfg *config.QueueWorkerConfig) {
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     cfg.Addr,
		DB:       cfg.Db,
		Password: cfg.Password,
		Username: cfg.Username,
	})
	qw := qmetrics.NewQueueMetricsCollector(inspector)
	m.Registery.MustRegister(
		m.IngestConsumedTotal,
		m.IngestErrorsTotal,
		m.IngestSuccessTotal,
		m.EventDispatchLatency,
		m.EventDeliveryLatency,
		qw,
	)
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
	workerLabel           = "worker"
)

func InitMetrics(cfg *config.MetricsConfig) *Metrics {
	if !cfg.Enabled {
		return &Metrics{
			IsEnabled: false,
		}
	}

	m := &Metrics{
		IsEnabled: true,
		Registery: Reg(),

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
			[]string{eventTypeLabel, listenerLabel, workerLabel},
		),
		IngestErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hookrelay_ingest_errors_total",
				Help: "Total number of errors during event ingestion",
			},
			[]string{eventTypeLabel, listenerLabel, workerLabel},
		),
		IngestSuccessTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hookrelay_ingest_success_total",
				Help: "Total number of successful event ingestion",
			},
			// []string{eventLabel, eventTypeLabel, listenerLabel},
			[]string{eventTypeLabel, listenerLabel, workerLabel},
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
			[]string{listenerLabel, workerLabel},
		),
		EventDispatchLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "hookrelay_event_dispatch_latency",
				Help: "Total time (in microsecond) an event spends after subscription has been found, in HookRelay.",
				// Buckets: prometheus.ExponentialBuckets(100, 2, 10),
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			// []string{eventLabel, eventTypeLabel, listenerLabel},
			[]string{listenerLabel, workerLabel},
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
			[]string{workerLabel},
		),
		WorkerThreadsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hookrelay_worker_threads_count",
				Help: "Total number of active workers threads in hookrelay",
			},
			[]string{workerLabel},
		),
	}

	return m
}

func (m *Metrics) RecordEndToEndLatency(ev *event.Event, wrkr string) {
	if !m.IsEnabled {
		return
	}

	d := time.Since(ev.CreatedAt)
	t := float64(d) / float64(time.Millisecond)
	m.EventDeliveryLatency.With(prometheus.Labels{listenerLabel: "http", workerLabel: wrkr}).Observe(t)
}

func (m *Metrics) RecordPreFlightLatency(ev *event.Event) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(ev.CreatedAt)
	t := float64(d) / float64(time.Millisecond)
	m.PreFlightLatency.With(prometheus.Labels{listenerLabel: "http"}).Observe(t)
}

func (m *Metrics) RecordDispatchLatency(createdAt *time.Time, wrkr string) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(*createdAt)
	t := float64(d) / float64(time.Millisecond)
	m.EventDispatchLatency.With(prometheus.Labels{listenerLabel: "http", workerLabel: wrkr}).Observe(t)
}

func (m *Metrics) RecordIngestLatency(createdAt time.Time) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(createdAt)
	t := float64(d) / float64(time.Millisecond)
	slog.Info("IngestLatency", slog.Duration("duration", d), slog.Float64("latency_ms", t))
	m.IngestLatency.With(prometheus.Labels{listenerLabel: "http"}).Observe(t)
}

func (m *Metrics) IncrementIngestTotal() {
	if !m.IsEnabled {
		return
	}
	m.IngestTotal.With(prometheus.Labels{listenerLabel: "http"}).Inc()
}

func (m *Metrics) IncrementIngestErrorsTotal(ev *event.Event, wrkr string) {
	if !m.IsEnabled {
		return
	}
	m.IngestErrorsTotal.With(prometheus.Labels{eventTypeLabel: ev.EventType, listenerLabel: "http", workerLabel: wrkr}).Inc()
}

func (m *Metrics) IncrementIngestSuccessTotal(ev *event.Event, wrkr string) {
	if !m.IsEnabled {
		return
	}
	m.IngestSuccessTotal.With(prometheus.Labels{eventTypeLabel: ev.EventType, listenerLabel: "http", workerLabel: wrkr}).Inc()
}

func (m *Metrics) IncrementIngestConsumedTotal(ev *event.Event, wrkr string) {
	if !m.IsEnabled {
		return
	}
	m.IngestConsumedTotal.With(prometheus.Labels{listenerLabel: "http", eventTypeLabel: ev.EventType, workerLabel: wrkr}).Inc()
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
	m.FanoutSize.With(prometheus.Labels{eventTypeLabel: ev.EventType}).Observe(float64(size))
}

func (m *Metrics) UpdateWorkerQueueSize(wrkr string, size int) {
	if !m.IsEnabled {
		return
	}
	m.WorkerQueueSize.With(prometheus.Labels{workerLabel: wrkr}).Set(float64(size))
}

func (m *Metrics) UpdateWorkerThreadCount(wrkr string, count int) {
	if !m.IsEnabled {
		return
	}
	m.WorkerThreadsTotal.With(prometheus.Labels{workerLabel: wrkr}).Set(float64(count))
}

func Reg() *prometheus.Registry {
	re.Do(func() {
		reg = prometheus.NewPedanticRegistry()
	})

	return reg
}
