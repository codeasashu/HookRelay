package cli

import (
	"flag"
	"log"

	"github.com/codeasashu/HookRelay/internal/config"
)

var (
	customCfgPath      = ""
	isWorker      bool = false
)

func Execute() {
	flag.StringVar(&customCfgPath, "c", "", "file path of the config file")
	flag.BoolVar(&isWorker, "worker", false, "Run in worker mode")
	flag.Parse()

	_, err := config.LoadConfig(customCfgPath, isWorker)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
}
