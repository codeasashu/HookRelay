package metrics

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func GetHandler() http.Handler {
	pr := Reg()
	return promhttp.HandlerFor(pr, promhttp.HandlerOpts{Registry: pr})
}

func (m *Metrics) InitApiRoutes() {
	handler := GetHandler()
	{

		v2 := m.router.Group("/metrics")
		v2.GET("", func(c *gin.Context) {
			handler.ServeHTTP(c.Writer, c.Request)
		})
	}
}
