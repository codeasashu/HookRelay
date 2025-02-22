package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/delivery"
	"github.com/codeasashu/HookRelay/internal/worker"
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

func getUnmarshalerMap() worker.UnmarshalerMap {
	mm := map[string]func([]byte) (worker.Task, error){
		worker.TypeEventDelivery: delivery.EventDeliveryUnmarshaler(),
	}
	return mm
}

func handleWorker(cmd *cobra.Command, args []string) error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGHUP)
	ctx := context.Background()
	wg := sync.WaitGroup{}

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

	wrk := worker.NewQueueServer(mainApp, getUnmarshalerMap())

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("staring queue worker")
		go wrk.StartServer(delivery.SaveDeliveries(mainApp))
	}()

	select {
	case <-ctx.Done():
		mainApp.Shutdown(ctx)
		wrk.Shutdown()
		break

	case <-sigs:
		slog.Info("shutting down worker...")
		mainApp.Shutdown(ctx)
		wrk.Shutdown()
		break
	}

	close(sigs)
	wg.Wait()
	return nil
}
