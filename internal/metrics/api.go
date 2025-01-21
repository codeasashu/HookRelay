package metrics

import (
	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func AddRoutes(server *api.ApiServer) {
	pr := Reg()
	handler := promhttp.HandlerFor(pr, promhttp.HandlerOpts{Registry: pr})
	{

		v2 := server.Router.Group("/metrics")
		v2.GET("", func(c *gin.Context) {
			handler.ServeHTTP(c.Writer, c.Request)
		})
	}
}
