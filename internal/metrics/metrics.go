package metrics

import (
	"log/slog"
	"sync"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
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
	IngestTotal         *prometheus.CounterVec
	IngestLatency       *prometheus.HistogramVec
	TargetLatency       *prometheus.HistogramVec
	DeliveryLatency     *prometheus.HistogramVec
	PreFlightLatency    *prometheus.HistogramVec
	SubscriberDbLatency *prometheus.HistogramVec
	DeliveryDbLatency   *prometheus.HistogramVec

	// Subscriptions metrics
	TotalDeliverables *prometheus.GaugeVec

	// Dispatcher metrics
	TotalDeliveries *prometheus.CounterVec

	// Worker Metrics
	LocalWorkerQueueSize *prometheus.GaugeVec
}

func NewMetrics(cfg *config.Config) *Metrics {
	m := InitMetrics(&cfg.Metrics)

	if m.IsEnabled {
		inspector := asynq.NewInspector(asynq.RedisClientOpt{
			Addr:     cfg.QueueWorker.Addr,
			DB:       cfg.QueueWorker.Db,
			Password: cfg.QueueWorker.Password,
			Username: cfg.QueueWorker.Username,
		})
		qw := qmetrics.NewQueueMetricsCollector(inspector)
		m.Registery.MustRegister(
			// Add the standard process and go metrics to the registry
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			collectors.NewGoCollector(),
			m.IngestTotal,
			m.IngestLatency,
			m.TotalDeliverables,
			m.TotalDeliveries,
			m.PreFlightLatency,
			m.TargetLatency,
			m.DeliveryLatency,
			m.LocalWorkerQueueSize,
			m.SubscriberDbLatency,
			m.DeliveryDbLatency,
			qw,
		)

	}
	return m
}

const (
	EventTypeLabel        = "event_type"
	DeliveryStatusLabel   = "status_code"
	ListenerLabel         = "listener"
	DeliveryLabel         = "deliver"
	OwnerLabel            = "owner_id"
	TargetUrlLabel        = "url"
	TargetMethodLabel     = "method"
	WorkerLabel           = "worker"
	DeliveryDbSourceLabel = "db_source"
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
				Name: "hookrelay_total_ingested",
				Help: "Total number of events ingested",
			},
			[]string{ListenerLabel},
		),
		IngestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "hookrelay_ingest_latency",
				Help: "Total time (in milliseconds) an event spends in HookRelay.",
				// Buckets: prometheus.ExponentialBuckets(100, 2, 10),
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			[]string{ListenerLabel},
		),
		TotalDeliverables: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hookrelay_total_subscriptions",
				Help: "Total number of active subscriptions in hookrelay",
			},
			[]string{EventTypeLabel},
		),
		PreFlightLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "hookrelay_event_preflight_latency",
				Help: "Total time (in milliseconds) an event spends before being dispatched in HookRelay.",
				// Buckets: prometheus.ExponentialBuckets(100, 2, 10),
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			[]string{ListenerLabel},
		),
		TotalDeliveries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hookrelay_total_deliveries",
				Help: "Total number of events delivered",
			},
			[]string{EventTypeLabel, WorkerLabel, DeliveryStatusLabel},
		),
		TargetLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hookrelay_target_latency",
				Help:    "Total time (in milliseconds) an event delivery spends in target network.",
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			[]string{OwnerLabel, TargetMethodLabel, TargetUrlLabel},
		),
		DeliveryLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hookrelay_e2e_latency",
				Help:    "Total time (in milliseconds) an event spends in HookRelay.",
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			[]string{OwnerLabel, EventTypeLabel, WorkerLabel, DeliveryStatusLabel},
		),
		LocalWorkerQueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hookrelay_worker_queue_size",
				Help: "Total number of items in the local worker queue",
			},
			[]string{WorkerLabel},
		),
		SubscriberDbLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hookrelay_subscriberdb_latency",
				Help:    "Total time (in milliseconds) to fetch subscribers from db",
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			[]string{OwnerLabel, EventTypeLabel},
		),
		DeliveryDbLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hookrelay_deliverydb_latency",
				Help:    "Total time (in milliseconds) to fetch subscribers from db",
				Buckets: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
			},
			[]string{DeliveryDbSourceLabel},
		),
	}

	return m
}

func (m *Metrics) IncrementIngestTotal(listener string) {
	if !m.IsEnabled {
		return
	}
	m.IngestTotal.With(prometheus.Labels{ListenerLabel: listener}).Inc()
}

func (m *Metrics) RecordIngestLatency(listener string, createdAt time.Time) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(createdAt)
	t := float64(d) / float64(time.Millisecond)
	slog.Info("IngestLatency", slog.Duration("duration", d), slog.Float64("latency_ms", t))
	m.IngestLatency.With(prometheus.Labels{ListenerLabel: listener}).Observe(t)
}

func (m *Metrics) UpdateTotalDeliverables(eventType string, count int) {
	if !m.IsEnabled {
		return
	}
	m.TotalDeliverables.With(prometheus.Labels{EventTypeLabel: eventType}).Set(float64(count))
}

func (m *Metrics) RecordPreFlightLatency(listener string, createdAt *time.Time) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(*createdAt)
	t := float64(d) / float64(time.Millisecond)
	m.PreFlightLatency.With(prometheus.Labels{ListenerLabel: listener}).Observe(t)
}

func (m *Metrics) IncTotalDeliveries(eventType string, wrkr string, statusCode string) {
	if !m.IsEnabled {
		return
	}
	m.TotalDeliveries.With(prometheus.Labels{EventTypeLabel: eventType, WorkerLabel: wrkr, DeliveryStatusLabel: statusCode}).Inc()
}

func (m *Metrics) UpdateWorkerQueueSize(size int) {
	if !m.IsEnabled {
		return
	}
	m.LocalWorkerQueueSize.With(prometheus.Labels{WorkerLabel: "local"}).Set(float64(size))
}

func (m *Metrics) RecordSubscriberDbLatency(owner, eventType string, createdAt *time.Time) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(*createdAt)
	t := float64(d) / float64(time.Millisecond)
	m.SubscriberDbLatency.With(prometheus.Labels{OwnerLabel: owner, EventTypeLabel: eventType}).Observe(t)
}

func (m *Metrics) RecordDeliveryDbLatency(dbSource string, createdAt *time.Time) {
	if !m.IsEnabled {
		return
	}
	d := time.Since(*createdAt)
	t := float64(d) / float64(time.Millisecond)
	m.DeliveryDbLatency.With(prometheus.Labels{DeliveryDbSourceLabel: dbSource}).Observe(t)
}

func Reg() *prometheus.Registry {
	re.Do(func() {
		reg = prometheus.NewPedanticRegistry()
	})

	return reg
}
