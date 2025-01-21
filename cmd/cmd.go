package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/codeasashu/HookRelay/internal/api"
	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/pkg/listener"
	"github.com/codeasashu/HookRelay/pkg/subscription"

	"golang.org/x/sync/errgroup"
)

var g errgroup.Group

func main() {
	cli.Execute()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	// Init once
	m := metrics.GetDPInstance()
	go m.StartGoroutineMonitor(ctx)

	disp := dispatcher.NewDispatcher()
	disp.Start()

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
		log.Println("shutting down...")
		apiServer.Shutdown(ctx)
		httpListenerServer.Shutdown(ctx)

		stop()
	}()

	if err := g.Wait(); err != nil {
		disp.Stop()
		log.Println("Error while waiting for goroutines to complete:", err)
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
