package delivery

import (
	"errors"
	"strconv"
	"time"

	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/event"
	"github.com/gin-gonic/gin"
)

func AddRoutes(server *api.ApiServer, db database.Database) {
	{
		v1 := server.Router.Group("/delivery")
		v1.GET(":owner_id", getDeliveriesHandler(db))
	}
}

//	func createDeliveryHandler(db database.Database) gin.HandlerFunc {
//		return func(c *gin.Context) {
//			ownerId := c.Param("owner_id")
//			if ownerId == "" {
//				c.JSON(400, gin.H{"status": "error", "error": "Invalid owner id"})
//				return
//			}
//			deliveries, err := event.GetDeliveriesByOwner(db.GetDB(), ownerId, 10, 0)
//			if err != nil {
//				c.JSON(500, gin.H{"status": "error", "error": err.Error()})
//				return
//			}
//			c.JSON(200, gin.H{"status": "success", "deliveries": deliveries})
//		}
//	}

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

		totalCount, deliveries, nextCursor, err := event.GetDeliveriesByOwner(
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
