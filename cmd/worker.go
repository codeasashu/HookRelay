package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/delivery"
	wrkr "github.com/codeasashu/HookRelay/internal/worker"
	"github.com/codeasashu/HookRelay/pkg/worker"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

func WorkerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start queue worker",
		RunE:  handleWorker,
	}
	return cmd
}

func getUnmarshalerMap() wrkr.UnmarshalerMap {
	mm := map[string]func([]byte) (wrkr.Task, error){
		wrkr.TypeEventDelivery: delivery.EventDeliveryUnmarshaler(),
	}
	return mm
}

func handleWorker(cmd *cobra.Command, args []string) error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGHUP)
	ctx := context.Background()

	cfgFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}
	skipWal, _ := cmd.Flags().GetBool("skip-wal")
	mainApp, err := app.FromConfig(cfgFile, ctx, skipWal)
	if err != nil {
		return err
	}
	mainApp.InitDeliveryDb()

	slog.Info("staring queue worker")
	wrk, err := worker.StartQueueWorker(mainApp, getUnmarshalerMap())
	if err != nil {
		return err
	}

	// Wait for termination signal.
	// <-sigs
	//
	// slog.Info("shutting down worker...")
	// wrk.Shutdown()
	//
	// return nil
	//

	select {
	case <-ctx.Done():
		mainApp.Shutdown(ctx)
		wrk.Shutdown()
		break

	case <-sigs:
		slog.Info("shutting down server...")
		mainApp.Shutdown(ctx)
		wrk.Shutdown()
		break
	}

	close(sigs)
	return nil
}
