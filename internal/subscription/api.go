package subscription

import (
	"log/slog"
	"net/http"

	"github.com/codeasashu/HookRelay/internal/target"
	"github.com/gin-gonic/gin"
)

func (s *Subscription) InitApiRoutes() {
	{
		v1 := s.router.Group("/subscriptions")
		v1.POST("", createSubscriberHandler(s))
	}
}

func createSubscriberHandler(s *Subscription) gin.HandlerFunc {
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
