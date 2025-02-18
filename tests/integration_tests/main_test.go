package integrationtests

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/codeasashu/HookRelay/internal/cli"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/dispatcher"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/codeasashu/HookRelay/tests/mock"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	fmt.Println("Starting server...")
	var wg sync.WaitGroup

	cli.GetAppInstance()
	ms := mock.InitMockHTTPServer(9092)
	ms.StartFixedDurationServer(1 * time.Millisecond)
	// cli.Execute()

	// Load custom Config
	_, err := config.LoadConfig("test_config.toml")
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	wp := &worker.WorkerPool{}
	disp := dispatcher.NewDispatcher(wp)
	// benchmark.NewLatencyBench(disp)
	// disp.Start()
	ctx, cancel := context.WithCancel(context.Background())
	s := RunHTTPListener(ctx, &wg, disp)

	fmt.Println("Started server...")

	// Wait for the server to start
	time.Sleep(2 * time.Second)

	exitCode := m.Run()

	cancel()
	s.Shutdown(ctx)
	// disp.Stop()

	// wg.Wait()
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
