package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	"github.com/codeasashu/HookRelay/internal/delivery"
	"github.com/codeasashu/HookRelay/internal/listener"
	"github.com/codeasashu/HookRelay/internal/subscription"
	"github.com/codeasashu/HookRelay/internal/wal"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/codeasashu/HookRelay/internal/app"
)

var OsExit = os.Exit

func ServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start server",
		RunE:  handleServer,
	}

	cmd.Flags().BoolP("legacy-mode", "l", false, "Use legacy code")
	cmd.Flags().Bool("skip-wal", false, "Don't use WAL mode")
	return cmd
}

func getMarshalerMap() worker.MarshalerMap {
	mm := map[string]func(worker.Task) ([]byte, error){
		worker.TypeEventDelivery: delivery.EventDeliveryMarshaler(),
	}
	return mm
}

func setupWorkers(f *app.HookRelayApp) *worker.WorkerPool {
	wp := worker.InitPool(f)
	localWorker := worker.CreateLocalWorker(f, wp, delivery.SaveDeliveries(f))
	wp.SetLocalClient(localWorker)
	queueWorker := worker.CreateQueueWorker(f, getMarshalerMap(), delivery.SaveDeliveries(f))
	wp.SetQueueClient(queueWorker)
	return wp
}

func handleServer(cmd *cobra.Command, args []string) error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGHUP)
	ctx := context.Background()
	wg := sync.WaitGroup{}
	serverErrCh := make(chan error, 2)

	cfgFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}
	skipWal, _ := cmd.Flags().GetBool("skip-wal")
	mainApp, err := app.FromConfig(cfgFile, ctx, skipWal)
	if err != nil {
		return err
	}
	mainApp.InitSubscriptionDb()
	mainApp.InitDeliveryDb()

	// Init worker pool
	wp := setupWorkers(mainApp)

	// Init HTTP delivery
	deliveryApp, err := delivery.NewHTTPDelivery(mainApp, wp)
	if err != nil {
		return fmt.Errorf("failed to initialize delivery: %v", err)
	}
	deliveryApp.InitApiRoutes()

	// Init subscription
	legacyMode, _ := cmd.Flags().GetBool("legacy-mode")
	subscriptionApp, err := subscription.NewSubscription(mainApp, legacyMode)
	if err != nil {
		return fmt.Errorf("failed to initialize delivery: %v", err)
	}
	subscriptionApp.InitApiRoutes()

	httpListener := listener.NewHTTPListener(mainApp, deliveryApp, subscriptionApp)
	httpListener.InitApiRoutes()

	wg.Add(1)
	go func() {
		defer wg.Done()
		go mainApp.Start(serverErrCh)
	}()

	// Init background housekeeping threads
	if mainApp.WAL != nil {
		cb := []func(db *sql.DB) error{}

		if mainApp.DeliveryDb != nil {
			cb = append(cb, delivery.ProcessRotatedWAL(mainApp.Metrics, mainApp.DeliveryDb))
		}

		wal.InitBG(mainApp.WAL, cb)
	}

	// Run indefinitely
	select {
	case <-ctx.Done():
		mainApp.Shutdown(ctx)
		httpListener.Shutdown(ctx)
		wp.Shutdown()

		wal.ShutdownBG()
		break

	case <-sigs:
		slog.Info("shutting down server...")
		mainApp.Shutdown(ctx)
		httpListener.Shutdown(ctx)
		wp.Shutdown()

		wal.ShutdownBG()
		break
	}

	close(serverErrCh)
	close(sigs)

	wg.Wait()
	return nil
}
