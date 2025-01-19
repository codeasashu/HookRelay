package subscription

import (
	"log"
	"net/http"

	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/gin-gonic/gin"
)

type createSubscription struct {
	*Subscription
	EventType string `json:"event_type" binding:"required"`
}

func AddRoutes(server *api.ApiServer) {
	{
		v1 := server.Router.Group("/subscriptions")
		v1.POST("", createSubscriptionHandler)
	}
}

func createSubscriptionHandler(c *gin.Context) {
	var subscription createSubscription
	if err := c.ShouldBindJSON(&subscription); err != nil {
		c.JSON(400, gin.H{"status": "error", "error": err.Error()})
		return
	}

	err := CreateSubscription(subscription.EventType, subscription.Subscription)
	if err != nil {
		c.JSON(400, gin.H{"status": "error", "error": err.Error()})
		return
	}
	log.Printf("Received Subscription for eventType=%s\n", subscription.ID)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
