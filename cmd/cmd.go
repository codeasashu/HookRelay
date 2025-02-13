package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/database"
	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/internal/logger"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/wal"
	wrkr "github.com/codeasashu/HookRelay/internal/worker"
	"github.com/codeasashu/HookRelay/pkg/delivery"
	"github.com/codeasashu/HookRelay/pkg/listener"
	"github.com/codeasashu/HookRelay/pkg/subscription"
	"github.com/codeasashu/HookRelay/pkg/worker"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

var (
	g          errgroup.Group
	wl         wal.AbstractWAL
	sigs       chan os.Signal
	deliveryDb database.Database
)

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
	sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGHUP)
	// Init metrics
	metrics.GetDPInstance()
	// Init Remote DB
	// db, err := database.NewPostgreSQLStorage()
	db, err := database.NewMySQLStorage(config.HRConfig.Subscription.Database)
	if err != nil {
		slog.Error("error connecting to db", "err", err)
		os.Exit(1)
	}
	app.DB = db

	// Delivery DB
	deliveryDb, err = database.NewMySQLStorage(config.HRConfig.Delivery.Database)
	if err != nil {
		slog.Warn("error connecting to delivery db, accounting will not work", "err", err)
	}
	acc := wal.NewAccounting(deliveryDb)
	wl = wal.NewSQLiteWAL(acc)
	if err := wl.Init(time.Now()); err != nil {
		slog.Error("could not initialize WAL", slog.Any("error", err))
		os.Exit(1)
	} else {
		wal.InitBG(wl, 100)
	}
}

func startWorkerQueueMode() {
	slog.Info("staring queue worker")
	metricsSrv := worker.StartMetricsServer()
	srv := worker.StartQueueWorker(sigs, wl)

	// Wait for termination signal.
	<-sigs

	slog.Info("shutting down worker...")
	// Stop worker server.
	srv.Shutdown()

	// Stop metrics server.
	if err := metricsSrv.Shutdown(context.Background()); err != nil {
		slog.Error("Error: metrics server shutdown", "err", err)
	}
}

func startServerMode(app *cli.App) {
	serverErrCh := make(chan error, 2)
	wg := sync.WaitGroup{}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	// Always start a local worker per server
	wp := wrkr.NewWorkerPool(app, wl)
	wp.AddQueueClient()

	disp := dispatcher.NewDispatcher(wp)

	apiServer := api.InitApiServer()

	subscription.AddRoutes(apiServer)
	delivery.AddRoutes(apiServer, deliveryDb)
	httpListenerServer := listener.NewHTTPListener(disp, wl)
	httpListenerServer.AddRoutes(apiServer)

	// Add prometheus api
	metrics.AddRoutes(apiServer)

	wg.Add(1)
	go func() {
		defer wg.Done()
		go apiServer.Start(serverErrCh)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-sigs
		stop()
	}()

	// Run indefinitely
	select {
	case <-ctx.Done():
		apiServer.Shutdown(ctx)
		httpListenerServer.Shutdown(ctx)

		// Shutdown WAL
		wal.ShutdownBG()
		stop()
		break

		// os.Exit(0)
	case <-sigs:
		slog.Info("shutting down server...")
		apiServer.Shutdown(ctx)
		httpListenerServer.Shutdown(ctx)

		// Shutdown WAL
		wal.ShutdownBG()
		stop()
	}

	close(serverErrCh) // Close the channel when both servers are done
	close(sigs)

	wg.Wait()
}
