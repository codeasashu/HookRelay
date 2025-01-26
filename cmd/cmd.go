package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/internal/logger"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/pkg/listener"
	"github.com/codeasashu/HookRelay/pkg/subscription"
	"github.com/codeasashu/HookRelay/pkg/worker"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

var g errgroup.Group

func main() {
	cli.Execute()
	slog.SetDefault(logger.New())

	// Init once
	metrics.GetDPInstance()

	if config.HRConfig.IsQueueWorker() {
		startWorkerQueueMode()
	} else if config.HRConfig.IsPubsubWorker() {
		worker.StartPubsubWorker()
	} else {
		startServerMode()
	}
}

func startWorkerQueueMode() {
	slog.Info("staring queue worker")
	sigs := make(chan os.Signal, 1)
	metricsSrv := worker.StartMetricsServer()
	srv := worker.StartQueueWorker(sigs)

	// Wait for termination signal.
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT)
	<-sigs

	// Stop worker server.
	srv.Shutdown()

	// Stop metrics server.
	if err := metricsSrv.Shutdown(context.Background()); err != nil {
		slog.Error("Error: metrics server shutdown", "err", err)
	}
}

func startServerMode() {
	slog.Info("Staring HookRelay...")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	disp := dispatcher.NewDispatcher()
	// disp.Start()

	apiServer := api.InitApiServer()
	// Add subscription API
	subscription.Init()
	subscription.AddRoutes(apiServer)

	// Add prometheus api
	metrics.AddRoutes(apiServer)

	// Finally start api server
	g.Go(apiServer.Start)

	httpListenerServer := listener.NewHTTPListener(":8082", disp)
	g.Go(httpListenerServer.StartAndReceive)

	go func() {
		<-ctx.Done()

		// Trigger shutdown handlers
		slog.Info("shutting down...")
		apiServer.Shutdown(ctx)
		httpListenerServer.Shutdown(ctx)

		stop()
	}()

	if err := g.Wait(); err != nil {
		// disp.Stop()
		if err != http.ErrServerClosed {
			slog.Error("Error while waiting for goroutines to complete.", slog.Any("error", err))
		}
		os.Exit(0)
	}

	// Run indefinitely
	select {}
}
