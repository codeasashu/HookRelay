package subscription

import (
	"net/http"

	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/middleware"
	"github.com/codeasashu/HookRelay/internal/subscription"
	"github.com/codeasashu/HookRelay/internal/target"
	"github.com/gin-gonic/gin"
)

func AddRoutes(server *api.ApiServer) {
	{
		v1 := server.Router.Group("/subscriptions")
		v1.POST("", createSubscriptionHandler(server.App))
	}
}

func createSubscriptionHandler(app *cli.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := middleware.FromContext(c.Request.Context())
		logger.Info("Processing subscription request")
		var cs *subscription.ReadSubscription
		if err := c.ShouldBindJSON(&cs); err != nil {
			c.JSON(400, gin.H{"status": "error", "error": err.Error()})
			return
		}

		s := &subscription.Subscription{
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
		model := subscription.NewSubscriptionModel(app.DB)
		err := model.CreateSubscription(s)
		if err != nil {
			switch err {
			case subscription.ErrSubscriptionExists:
				c.JSON(409, gin.H{"status": "error", "error": err.Error()})
				return
			case subscription.ErrSubscriptionNotCreated:
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
