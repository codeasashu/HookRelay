package integrationtests

import (
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/codeasashu/HookRelay/internal/cli"
)

func TestMain(m *testing.M) {
	log.SetOutput(os.Stdout)
	// var wg sync.WaitGroup
	//
	// cli.GetAppInstance()
	// ms := mock.InitMockHTTPServer(9092)
	// ms.StartFixedDurationServer(1 * time.Millisecond)
	// // cli.Execute()
	//
	// // Load custom Config
	// _, err := config.LoadConfig("test_config.toml")
	// if err != nil {
	// 	log.Fatalf("Error loading configuration: %v", err)
	// }
	//
	// wp := &worker.WorkerPool{}
	// disp := dispatcher.NewDispatcher(wp)
	// // benchmark.NewLatencyBench(disp)
	// // disp.Start()
	// ctx, cancel := context.WithCancel(context.Background())
	// s := RunHTTPListener(ctx, &wg, disp)
	//
	// fmt.Println("Started server...")
	//
	//

	os.Args = []string{"HookRelay", "server", "-c", "test_config.toml"}
	app := cli.GetAppInstance()
	app.Logger = slog.Default()
	c := cli.NewCli(app)
	c.Setup()

	// Wait for the server to start
	time.Sleep(2 * time.Second)

	m.Run()

	// cancel()
	// s.Shutdown(ctx)
	// // disp.Stop()
	//
	// // wg.Wait()
	// if exitCode != 0 {
	// 	os.Exit(exitCode)
	// }
}
