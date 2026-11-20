package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"goto_splitter/config"
	"goto_splitter/coordinator"
	"goto_splitter/ui"
	"goto_splitter/utils/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize the logger
	if err := logging.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

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
	if startupErr := c.StartupDefinitionError(); startupErr != nil {
		logging.GetLogger().Error("game definition startup error: %v", startupErr)
	}
	logging.GetLogger().Info("GotoSplitter runtime started; LiveSplit server listening at %s", c.ServerURL())

	if err := ui.RunUI(ctx, cfg, c.StartupDefinitionError(), c.SetGameDefinition, c.ResetTimerWithOptions, func(sniStatus, wsStatus string) {}); err != nil {
		logging.GetLogger().Error("UI stopped with an error: %v", err)
	}
	stop()
	logging.GetLogger().Info("shutting down GotoSplitter")
	if err := c.Stop(); err != nil {
		logging.GetLogger().Error("failed to stop coordinator cleanly: %v", err)
	}
}

func failStartup(ctx context.Context, startupErr error) {
	logging.GetLogger().Error("fatal startup error: %v", startupErr)
	if err := ui.ShowStartupError(ctx, startupErr); err != nil {
		logging.GetLogger().Error("could not display startup error in UI: %v", err)
	}
	os.Exit(1)
}
