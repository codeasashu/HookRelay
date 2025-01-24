package metrics

import (
	"net/http"

	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func GetHandler() http.Handler {
	pr := Reg()
	return promhttp.HandlerFor(pr, promhttp.HandlerOpts{Registry: pr})
}

func AddRoutes(server *api.ApiServer) {
	handler := GetHandler()
	{

		v2 := server.Router.Group("/metrics")
		v2.GET("", func(c *gin.Context) {
			handler.ServeHTTP(c.Writer, c.Request)
		})
	}
}
