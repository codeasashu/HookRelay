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
	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/internal/logger"
	"github.com/codeasashu/HookRelay/internal/metrics"
	wrkr "github.com/codeasashu/HookRelay/internal/worker"
	"github.com/codeasashu/HookRelay/pkg/listener"
	"github.com/codeasashu/HookRelay/pkg/subscription"
	"github.com/codeasashu/HookRelay/pkg/worker"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

var g errgroup.Group

func main() {
	app := cli.GetAppInstance()
	app.Logger = slog.Default()
	c := cli.NewCli(app)
	_, err := c.Setup()
	if err != nil {
		slog.Error("Error: ", "err", err)
		os.Exit(1)
	}

	slog.SetDefault(logger.New())
	Init(app)

	if app.IsWorker {
		startWorkerQueueMode()
	} else {
		startServerMode(app)
	}
}

func Init(app *cli.App) {
	// Init metrics
	metrics.GetDPInstance()

	// Init DB
	// db, err := database.NewPostgreSQLStorage()
	db, err := database.NewMySQLStorage()
	if err != nil {
		slog.Error("error connecting to db", "err", err)
		os.Exit(1)
	}
	app.DB = db
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

func startServerMode(app *cli.App) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	// Always start a local worker per server
	wp := wrkr.NewWorkerPool(app)
	wp.AddQueueClient()

	disp := dispatcher.NewDispatcher(wp)

	apiServer := api.InitApiServer()

	subscription.AddRoutes(apiServer)
	httpListenerServer := listener.NewHTTPListener(disp)
	httpListenerServer.AddRoutes(apiServer)

	// Add prometheus api
	metrics.AddRoutes(apiServer)

	// Finally start api server
	g.Go(apiServer.Start)
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
