package delivery

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/database"
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
	db      database.Database
	wp      *worker.WorkerPool
	wl      wal.AbstractWAL
}

func NewHTTPDelivery(a *app.HookRelayApp, wp *worker.WorkerPool) (*HTTPDelivery, error) {
	return &HTTPDelivery{db: a.DeliveryDb, router: a.Router, metrics: a.Metrics, wp: wp, wl: a.WAL}, nil
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

func (d *HTTPDelivery) InitApiRoutes() {
	{
		v1 := d.router.Group("/delivery")
		v1.GET(":owner_id", getDeliveriesHandler(d.db))
	}
}

func parseTimeParam(param string, endOfDay bool) (*time.Time, error) {
	if param == "" {
		return nil, errors.New("empty date")
	}

	// Try parsing full RFC3339 timestamp
	parsed, err := time.Parse(time.RFC3339, param)
	if err == nil {
		return &parsed, nil
	}

	// Try parsing as YYYY-MM-DD (date only)
	parsed, err = time.Parse("2006-01-02", param)
	if err == nil {
		if endOfDay {
			parsed = parsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
		return &parsed, nil
	}

	return nil, errors.New("invalid date format. allowed formats are: RFC3339, YYYY-MM-DD")
}

func getDeliveriesHandler(db database.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		ownerId := c.Param("owner_id")
		if ownerId == "" {
			c.JSON(400, gin.H{"status": "error", "error": "Invalid owner ID"})
			return
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		cursor, _ := strconv.Atoi(c.DefaultQuery("cursor", "0"))

		eventType := c.Query("event_type")
		createdGteStr := c.Query("created_gte")
		createdLteStr := c.Query("created_lte")
		createdAtGte, err := parseTimeParam(createdGteStr, false)
		if err != nil && createdGteStr != "" {
			c.JSON(400, gin.H{"status": "error", "error": "Invalid created_gte value: " + err.Error()})
			return
		}
		createdAtLte, err := parseTimeParam(createdLteStr, true)
		if err != nil && createdLteStr != "" {
			c.JSON(400, gin.H{"status": "error", "error": "Invalid created_lte value: " + err.Error()})
			return
		}

		totalCount, deliveries, nextCursor, err := GetDeliveriesByOwner(
			db.GetDB(), ownerId, &eventType, createdAtGte, createdAtLte, int64(cursor), uint16(limit),
		)
		if err != nil {
			c.JSON(500, gin.H{"status": "error", "error": err.Error()})
			return
		}

		response := gin.H{
			"status":      "success",
			"count":       totalCount,
			"next_cursor": &nextCursor,
			"deliveries":  deliveries,
		}

		c.JSON(200, response)
	}
}
