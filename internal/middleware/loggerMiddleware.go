package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Define a context key to store/retrieve the logger
type loggerKeyType string

const (
	loggerKey         loggerKeyType = "logger"
	traceIDHeaderName string        = "X-Hookrelay-Trace-Id"
)

// WithLogger stores the logger in context
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves the logger from context
func FromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerKey).(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return logger
}

// LoggerMiddleware adds a unique trace ID to each request and response
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		// Fetch trace ID from the request headers and trim whitespaces
		traceID := strings.TrimSpace(c.GetHeader(traceIDHeaderName))

		// If trace ID is missing, generate a new one
		if traceID == "" {
			traceID = uuid.NewString()
		}

		// Create a logger with the trace ID
		logger := slog.Default().With("trace_id", traceID)

		// Store logger in Gin context and request context
		c.Set("logger", logger)
		c.Request = c.Request.WithContext(WithLogger(c.Request.Context(), logger))

		// Set the trace ID in response headers
		c.Writer.Header().Set(traceIDHeaderName, traceID)

		logger.Info("Incoming request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query_params", c.Request.URL.RawQuery,
			"request_uri", c.Request.RequestURI,
			"request_headers", fmt.Sprintf("%v", c.Request.Header),
		)

		// Process request (calls the next middleware or handler)
		c.Next()

		logger.Info("Request completed", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status())
	}
}

// isValidUUID checks if a string is a valid UUID format
// func isValidUUID(id string) bool {
// 	_, err := uuid.Parse(strings.TrimSpace(id))
// 	return err == nil
// }
