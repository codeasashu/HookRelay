package cli

import (
	"flag"
	"log"

	"github.com/codeasashu/HookRelay/internal/config"
)

var customCfgPath = ""

func Execute() {
	flag.StringVar(&customCfgPath, "c", "", "file path of the config file")
	flag.Parse()

	_, err := config.LoadConfig(customCfgPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
}
