package subscription

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/codeasashu/HookRelay/internal/target"
	"github.com/gin-gonic/gin"
)

func (s *Subscription) InitApiRoutes() {
	{
		v1 := s.router.Group("/subscriptions")
		v1.GET("", getSubscriptions(s))
		v1.PUT("/:id", updateSubscription(s))
		v1.DELETE("/:id", deleteSubscription(s))
		v1.POST("", createSubscriber(s))
	}
}

func parseLimitOffset(c *gin.Context) (int, int) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	maxLimit := 100
	_limit := 10
	_offset := 0
	var err error

	parsedLimit, err := strconv.Atoi(limitStr)
	if err != nil {
		slog.Warn("Invalid 'limit' parameter, using default.", "param_value", limitStr, "default_value", _limit, "error", err)
	} else {
		_limit = parsedLimit
	}

	parsedOffset, err := strconv.Atoi(offsetStr)
	if err != nil {
		slog.Warn("Invalid 'offset' parameter, using default.", "param_value", offsetStr, "default_value", _offset, "error", err)
	} else {
		_offset = parsedOffset
	}

	if _limit <= 0 {
		slog.Warn("Parsed 'limit' is not positive, adjusting to default.", "parsed_limit", _limit, "new_default", 10)
		_limit = 10 // Or some other sensible default minimum
	}
	if _limit > maxLimit {
		slog.Warn("Parsed 'limit' exceeds maximum, adjusting to max.", "parsed_limit", _limit, "max_limit", maxLimit)
		_limit = maxLimit
	}

	if _offset < 0 {
		slog.Warn("Parsed 'offset' is negative, adjusting to 0.", "parsed_offset", _offset)
		_offset = 0
	}
	return _limit, _offset
}

func getSubscriptions(s *Subscription) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, offset := parseLimitOffset(c)
		ownerId := c.DefaultQuery("ownerid", "")
		eventType := c.DefaultQuery("event_type", "")
		subscribers, hasMore, err := s.GetSubscribers(ownerId, eventType, limit, offset)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if subscribers == nil {
			subscribers = []Subscriber{}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "data": subscribers, "more": hasMore})
	}
}

func updateSubscription(s *Subscription) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(400, gin.H{"status": "error", "error": "id is required"})
			return
		}
		var cs *ReadSubscriber
		if err := c.ShouldBindJSON(&cs); err != nil {
			c.JSON(400, gin.H{"status": "error", "error": err.Error()})
			return
		}

		slog.Info("update subscription", "cs", cs)

		sub := &Subscriber{
			ID:      cs.ID,
			OwnerId: cs.OwnerId,
			Target: &target.Target{
				Type:        target.TargetType(target.TargetHTTP),
				HTTPDetails: cs.Target,
			},
			EventTypes: cs.EventTypes,
			Tags:       cs.Tags,
			Status:     cs.Status,
			CreatedAt:  cs.CreatedAt,
		}

		updated, err := s.UpdateSubscriber(id, sub)
		if err != nil {
			switch err {
			case ErrSubscriptionExists:
				c.JSON(409, gin.H{"status": "error", "error": err.Error()})
				return
			case ErrLegacySubscription:
				c.JSON(409, gin.H{"status": "error", "error": err.Error()})
				return
			case ErrSubscriptionNotCreated:
				c.JSON(500, gin.H{"status": "error", "error": err.Error()})
				return
			default:
				c.JSON(400, gin.H{"status": "error", "error": err.Error()})
				return
			}
		}
		if !updated {
			c.JSON(200, gin.H{"status": "success", "error": "not change found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func deleteSubscription(s *Subscription) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(400, gin.H{"status": "error", "error": "id is required"})
			return
		}
		deleted, err := s.DeleteSubscriber(id)
		if err != nil {
			c.JSON(500, gin.H{"status": "error", "error": err.Error()})
			return
		}
		if !deleted {
			c.JSON(404, gin.H{"status": "error", "error": "subscription not found"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	}
}

func createSubscriber(s *Subscription) gin.HandlerFunc {
	return func(c *gin.Context) {
		var cs *ReadSubscriber
		if err := c.ShouldBindJSON(&cs); err != nil {
			c.JSON(400, gin.H{"status": "error", "error": err.Error()})
			return
		}

		slog.Info("createSubscriberHandler", "cs", cs)

		sub := &Subscriber{
			ID:      cs.ID,
			OwnerId: cs.OwnerId,
			Target: &target.Target{
				Type:        target.TargetType(target.TargetHTTP),
				HTTPDetails: cs.Target,
			},
			EventTypes: cs.EventTypes,
			Tags:       cs.Tags,
			Status:     cs.Status,
			CreatedAt:  cs.CreatedAt,
		}
		err := s.CreateSubscriber(sub)
		if err != nil {
			switch err {
			case ErrSubscriptionExists:
				c.JSON(409, gin.H{"status": "error", "error": err.Error()})
				return
			case ErrLegacySubscription:
				c.JSON(409, gin.H{"status": "error", "error": err.Error()})
				return
			case ErrSubscriptionNotCreated:
				c.JSON(500, gin.H{"status": "error", "error": err.Error()})
				return
			default:
				c.JSON(400, gin.H{"status": "error", "error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}
