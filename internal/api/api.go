package api

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/gin-gonic/gin"
)

type ApiServer struct {
	Router *gin.Engine
	server *http.Server
}

func InitApiServer() *ApiServer {
	apiPort := config.HRConfig.Api.Port
	router := gin.Default()
	s := &http.Server{
		Addr:         ":" + strconv.Itoa(apiPort),
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return &ApiServer{
		Router: router,
		server: s,
	}
}

func (a *ApiServer) Start() error {
	slog.Info("Starting API server", "addr", a.server.Addr)
	return a.server.ListenAndServe()
}

func (a *ApiServer) Shutdown(ctx context.Context) {
	slog.Warn("Shutting down API server...")
	if err := a.server.Shutdown(ctx); err != nil {
		log.Fatal("API Server forced to shutdown:", err)
	}
}
