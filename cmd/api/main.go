package main

import (
	"context"
	"log"
	"os"

	"github.com/vadim/neo-metric/internal/app"
	"github.com/vadim/neo-metric/internal/config"
)

func main() {
	// Load configuration
	cfg := config.MustLoad()

	// Create root context
	ctx := context.Background()

	// Initialize application
	application, err := app.NewApp(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to initialize application: %v", err)
	}

	// Run application (blocks until shutdown)
	if err := application.Run(ctx); err != nil {
		log.Printf("application error: %v", err)
		os.Exit(1)
	}
}
