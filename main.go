package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"goto_splitter/config"
	"goto_splitter/coordinator"
	"goto_splitter/ui"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		failStartup(ctx, fmt.Errorf("failed to load configuration: %w", err))
		return
	}
	c := coordinator.NewCoordinator(cfg)
	if err := c.Start(ctx); err != nil {
		failStartup(ctx, fmt.Errorf("failed to start coordinator: %w", err))
		return
	}
	log.Printf("GotoSplitter runtime started; LiveSplit server listening at %s", c.ServerURL())

	if err := ui.RunUI(ctx, cfg, c.SetGameDefinition, c.ResetTimerWithOptions); err != nil {
		log.Printf("UI stopped with an error: %v", err)
	}
	stop()
	log.Println("shutting down GotoSplitter")
	if err := c.Stop(); err != nil {
		log.Printf("failed to stop coordinator cleanly: %v", err)
	}
}

func failStartup(ctx context.Context, startupErr error) {
	log.Printf("fatal startup error: %v", startupErr)
	if err := ui.ShowStartupError(ctx, startupErr); err != nil {
		log.Printf("could not display startup error in UI: %v", err)
	}
	os.Exit(1)
}
