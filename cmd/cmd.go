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
	"github.com/codeasashu/HookRelay/internal/event"
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
	sigs       chan os.Signal
	g          errgroup.Group
	deliveryDb database.Database
)

func main() {
	// Create some signals for graceful shutdown
	sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGHUP)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	app := initCLI(ctx)
	slog.SetDefault(logger.New()) // Init logging
	metrics.GetDPInstance()       // Init metrics

	initDbs(app)
	accounting := wal.NewAccounting(deliveryDb) // Init accounting

	if app.IsWorker {
		startWorkerQueueMode(ctx, accounting)
	} else {
		startServerMode(app, ctx, accounting)
	}
}

func initCLI(ctx context.Context) *cli.App {
	app := cli.GetAppInstance()
	app.Logger = slog.Default()
	c := cli.NewCli(app)
	_, err := c.Setup()
	if err != nil {
		slog.Error("Error: ", "err", err)
		os.Exit(1)
	}
	app.Ctx = ctx
	return app
}

func initDbs(app *cli.App) {
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
}

func startWorkerQueueMode(ctx context.Context, accounting *wal.Accounting) {
	slog.Info("staring queue worker")
	wrk, err := worker.StartQueueWorker(ctx, accounting)
	if err != nil {
		return
	}

	// Wait for termination signal.
	<-sigs

	slog.Info("shutting down worker...")
	wrk.Shutdown()
}

func initWAL(accounting *wal.Accounting) wal.AbstractWAL {
	wl := wal.NewSQLiteWAL()

	if err := wl.Init(time.Now()); err != nil {
		slog.Error("could not initialize WAL", slog.Any("error", err))
		os.Exit(1)
	}

	// Do accounting with WAL
	walCallbacks := []func([]*event.EventDelivery){
		accounting.CreateDeliveries,
	}

	// Init background housekeeping threads
	wal.InitBG(wl, 100, walCallbacks)
	return wl
}

func initServerWorkerPool(app *cli.App, ctx context.Context, wl wal.AbstractWAL) *wrkr.WorkerPool {
	wp := &wrkr.WorkerPool{}
	wp.AddLocalClient(app, ctx, wl)
	// For buffered events, incase local workers are overwhelmed
	wp.AddQueueClient()
	return wp
}

func startServerMode(app *cli.App, ctx context.Context, accounting *wal.Accounting) {
	serverErrCh := make(chan error, 2)
	wg := sync.WaitGroup{}

	// WAL is initialised in the main program
	wl := initWAL(accounting)

	// Init server WorkerPool with local workers
	wp := initServerWorkerPool(app, ctx, wl)

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

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	<-sigs
	// 	stop()
	// }()
	//
	// Run indefinitely
	select {
	case <-ctx.Done():
		apiServer.Shutdown(ctx)
		httpListenerServer.Shutdown(ctx)

		// Shutdown WAL
		wal.ShutdownBG()
		// stop()
		break

		// os.Exit(0)
	case <-sigs:
		slog.Info("shutting down server...")
		apiServer.Shutdown(ctx)
		httpListenerServer.Shutdown(ctx)

		// Shutdown WAL
		wal.ShutdownBG()
		// stop()
	}

	close(serverErrCh) // Close the channel when both servers are done
	close(sigs)

	wg.Wait()
}
