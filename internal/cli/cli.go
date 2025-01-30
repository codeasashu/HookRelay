package cli

import (
	"log/slog"
	"os"

	// "github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type HookRelayCli struct {
	cmd      *cobra.Command
	app      *App
	IsWorker bool
}

func NewCli(app *App) *HookRelayCli {
	cmd := &cobra.Command{
		Use:     "HookRelay",
		Version: app.Version,
		Short:   "High Performance Webhooks Gateway",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.Help()
				os.Exit(0)
			}
		},
	}

	return &HookRelayCli{cmd: cmd, app: app}
}

func (c *HookRelayCli) Flags() *pflag.FlagSet {
	return c.cmd.PersistentFlags()
}

func (c *HookRelayCli) PersistentPreRunE(fn func(*cobra.Command, []string) error) {
	c.cmd.PersistentPreRunE = fn
}

func (c *HookRelayCli) PersistentPostRunE(fn func(*cobra.Command, []string) error) {
	c.cmd.PersistentPostRunE = fn
}

func (c *HookRelayCli) AddCommand(subCmd *cobra.Command) {
	c.cmd.AddCommand(subCmd)
}

func (c *HookRelayCli) Execute() error {
	return c.cmd.Execute()
}

func (c *HookRelayCli) Setup() (*App, error) {
	var configFile string
	c.Flags().StringVar(&configFile, "config", "", "Configuration file for hookrelay")
	c.AddCommand(AddServerCmd(c))
	c.AddCommand(AddWorkerCmd(c))
	if err := c.Execute(); err != nil {
		slog.Error("error parsing cli", "err", err)
		return nil, err
	}
	// c.app.Config =
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		c.app.Logger.Error("Error loading configuration", "err", err)
		return nil, err
	}
	c.app.Config = cfg
	c.app.IsWorker = c.IsWorker
	return c.app, nil
}

func AddWorkerCmd(c *HookRelayCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start worker instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			c.IsWorker = true
			c.app.Logger.Info("starting worker...")
			return nil
		},
	}
	return cmd
}

func AddServerCmd(c *HookRelayCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start server instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			c.IsWorker = false
			c.app.Logger.Info("starting server...")
			return nil
		},
	}
	return cmd
}

// func Execute() {
// 	flag.StringVar(&customCfgPath, "c", "", "file path of the config file")
// 	flag.BoolVar(&isQueueWorker, "queue", false, "Run worker in queue mode")
// 	flag.BoolVar(&isPubSubWorker, "pubsub", false, "Run worker in pubsub mode")
// 	flag.Parse()
//
// 	_, err := config.LoadConfig(customCfgPath, getWorkerType())
// 	if err != nil {
// 		log.Fatalf("Error loading configuration: %v", err)
// 	}
// }
//
// func getWorkerType() string {
// 	// Only one worker type is allowed
// 	if isQueueWorker && isPubSubWorker {
// 		log.Fatalf("Only one of --queue or --pubsub can be specified")
// 	}
//
// 	workerType := ""
// 	if isQueueWorker {
// 		workerType = string(config.QueueWorker)
// 	} else if isPubSubWorker {
// 		workerType = string(config.PubSubWorker)
// 	}
// 	return workerType
// }
