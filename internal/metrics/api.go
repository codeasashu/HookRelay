package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (m *Metrics) MetricsMiddleware(r *gin.Engine) {
	handler := promhttp.HandlerFor(m.Registery, promhttp.HandlerOpts{Registry: m.Registery})
	v2 := r.Group("/metrics")
	v2.GET("", func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	})
}
