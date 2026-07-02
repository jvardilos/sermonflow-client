package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sermonflow-client/internal/config"
	"sermonflow-client/internal/listener"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := listener.Run(ctx, cfg); err != nil {
		logger.Error("listener exited", "error", err)
		os.Exit(1)
	}
}
