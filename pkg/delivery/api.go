package delivery

import (
	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/gin-gonic/gin"
)

func AddRoutes(server *api.ApiServer, db database.Database) {
	{
		v1 := server.Router.Group("/delivery")
		v1.GET(":owner_id", createDeliveryHandler(db))
	}
}

func createDeliveryHandler(db database.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		ownerId := c.Param("owner_id")
		if ownerId == "" {
			c.JSON(400, gin.H{"status": "error", "error": "Invalid owner id"})
			return
		}
		deliveries, err := event.GetDeliveriesByOwner(db.GetDB(), ownerId, 10, 0)
		if err != nil {
			c.JSON(500, gin.H{"status": "error", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "success", "deliveries": deliveries})
	}
}
