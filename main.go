package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"goto_splitter/config"
	"goto_splitter/livesplit"
)

func main() {
	cfg := config.Load()
	server := livesplit.NewServer(cfg.LiveSplitAddress, func(connected bool, count int) {
		log.Printf("LiveSplit One clients connected: %d (connected: %t)", count, connected)
	})
	if err := server.Start(); err != nil {
		log.Fatalf("failed to start LiveSplit One server: %v", err)
	}
	log.Printf("LiveSplit One WebSocket server listening at %s", server.URL())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
	case err := <-server.Errors():
		log.Printf("LiveSplit One server stopped unexpectedly: %v", err)
	}

	if err := server.Stop(); err != nil {
		log.Printf("failed to stop LiveSplit One server cleanly: %v", err)
	}
}
