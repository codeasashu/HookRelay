package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LoggerMiddleware adds a unique trace ID to each request and response
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		// Fetch trace ID from the request headers and trim whitespaces
		traceID := strings.TrimSpace(c.GetHeader(string(config.TraceIDHeaderKey)))

		// If trace ID is missing, generate a new one
		if traceID == "" {
			traceID = uuid.NewString()
		}

		// Set the trace ID in request headers
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), config.TraceIDKey, traceID))

		// Set the trace ID in response headers
		c.Writer.Header().Set(string(config.TraceIDHeaderKey), traceID)

		slog.InfoContext(c, "Incoming request", "method", c.Request.Method, "path", c.Request.URL.Path, "query_params", c.Request.URL.RawQuery, "request_uri", c.Request.RequestURI, "request_headers", fmt.Sprintf("%v", c.Request.Header))

		// Process request (calls the next middleware or handler)
		c.Next()

		slog.InfoContext(c, "Request completed", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status())
	}
}

// isValidUUID checks if a string is a valid UUID format
// func isValidUUID(id string) bool {
// 	_, err := uuid.Parse(strings.TrimSpace(id))
// 	return err == nil
// }
