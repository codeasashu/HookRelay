package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/codeasashu/HookRelay/internal/benchmark"
	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/pkg/listener"
	"github.com/codeasashu/HookRelay/pkg/subscription"

	"golang.org/x/sync/errgroup"
)

var g errgroup.Group

func main() {
	cli.Execute()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	disp := dispatcher.NewDispatcher()
	lat := benchmark.NewLatencyBench(disp)
	disp.Start()

	apiServer := api.InitApiServer()
	// Add subscription API
	subscription.Init()
	subscription.AddRoutes(apiServer)

	// Finally start api server
	g.Go(apiServer.Start)
	// dispatcherPool := worker.NewWorkerPool(worker.MaxDispatchWorkers, worker.MaxQueueSize)
	// dispatcherPool.Start(worker.DispatchWorker)

	// go worker.DynamicWorkerScaler(dispatcherPool, 1*time.Second)

	httpListenerServer := listener.NewHTTPListener(":8082", disp)
	g.Go(httpListenerServer.StartAndReceive)

	go func() {
		<-ctx.Done()
		log.Println("shutting down...")

		// Trigger shutdown handlers
		apiServer.Shutdown(ctx)
		httpListenerServer.Shutdown(ctx)

		// disp.Stop()
		// Cancel all goroutines by stopping the errgroup context
		stop()
	}()

	if err := g.Wait(); err != nil {
		disp.Stop()
		lat.PrintSummary()
		log.Fatal(err)
	}

	// <-ctx.Done()
	// log.Printf("shutddown down...")

	// disp.Stop()
	// httpListenerServer.StartAndReceive()
	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.

	// Run indefinitely
	select {}
}
