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
	"github.com/codeasashu/HookRelay/internal/publisher"
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

	// Init worker pool
	wp := worker.InitPool(mainApp)
	localWorker := worker.NewLocalWorker(mainApp, wp)
	wp.SetLocalClient(localWorker)
	queueWorker := worker.NewQueueWorker(mainApp, getMarshalerMap())
	wp.SetQueueClient(queueWorker)

	// Add publishers to worker result threada
	billingPublisher := publisher.NewBillingPublisher(mainApp, &mainApp.Cfg.Billing)
	billingPublisher.SQSPublisher(ctx, localWorker.Fanout)
	if mainApp.WAL != nil {
		publisher.WALPublisher(mainApp.WAL, localWorker.Fanout)
	}

	// Init HTTP delivery
	deliveryApp, err := delivery.NewHTTPDelivery(mainApp, wp)
	if err != nil {
		return fmt.Errorf("failed to initialize delivery: %v", err)
	}

	// Init subscription
	subscriptionApp, err := subscription.NewSubscription(mainApp)
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
		wal.InitBG(mainApp.WAL, cb)
	}

	// Run indefinitely
	select {
	case <-ctx.Done():
		mainApp.Shutdown(ctx)
		httpListener.Shutdown(ctx)
		wp.Shutdown()
		billingPublisher.Shutdown()

		if mainApp.WAL != nil {
			wal.ShutdownBG()
		}
		break

	case <-sigs:
		slog.Info("shutting down server...")
		mainApp.Shutdown(ctx)
		httpListener.Shutdown(ctx)
		wp.Shutdown()
		billingPublisher.Shutdown()

		if mainApp.WAL != nil {
			wal.ShutdownBG()
		}
		break
	}

	close(serverErrCh)
	close(sigs)

	wg.Wait()
	return nil
}
