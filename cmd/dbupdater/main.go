package main

import (
	"log"
	"os"

	"dbupdater/config"
	"dbupdater/internal/core"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Config error: %s", err)
	}

	core.Run(cfg)
	os.Exit(0)
}
