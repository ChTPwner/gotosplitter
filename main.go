package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"goto_splitter/config"
	"goto_splitter/coordinator"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}
	c := coordinator.NewCoordinator(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := c.Start(ctx); err != nil {
		log.Fatalf("failed to start coordinator: %v", err)
	}
	log.Printf("GotoSplitter runtime started; LiveSplit server listening at %s", c.ServerURL())

	<-ctx.Done()
	log.Println("shutting down GotoSplitter")
	if err := c.Stop(); err != nil {
		log.Printf("failed to stop coordinator cleanly: %v", err)
	}
}
