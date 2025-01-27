package cli

import (
	"flag"
	"log"

	"github.com/codeasashu/HookRelay/internal/config"
)

var (
	customCfgPath       = ""
	isQueueWorker  bool = false
	isPubSubWorker bool = false
)

func Execute() {
	flag.StringVar(&customCfgPath, "c", "", "file path of the config file")
	flag.BoolVar(&isQueueWorker, "queue", false, "Run worker in queue mode")
	flag.BoolVar(&isPubSubWorker, "pubsub", false, "Run worker in pubsub mode")
	flag.Parse()

	_, err := config.LoadConfig(customCfgPath, getWorkerType())
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
}

func getWorkerType() string {
	// Only one worker type is allowed
	if isQueueWorker && isPubSubWorker {
		log.Fatalf("Only one of --queue or --pubsub can be specified")
	}

	workerType := ""
	if isQueueWorker {
		workerType = string(config.QueueWorker)
	} else if isPubSubWorker {
		workerType = string(config.PubSubWorker)
	}
	return workerType
}
