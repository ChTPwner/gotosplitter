package examples

import (
	"context"
	"log"
	"sync"
	"time"
)

type SplitAction string

const (
	ActionStart SplitAction = "start"
	ActionSplit SplitAction = "split"
	ActionReset SplitAction = "reset"
)

type Coordinator struct {
	actions chan SplitAction
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		actions: make(chan SplitAction, 16),
		stopCh:  make(chan struct{}),
	}
}

// RunLoop demonstrates a non-blocking, ticker-based polling orchestrator.
func (c *Coordinator) RunLoop(ctx context.Context, pollInterval time.Duration) {
	c.wg.Add(1)
	defer c.wg.Done()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[Coordinator] Shutting down main pipeline...")
			return

		case action := <-c.actions:
			log.Printf("[Coordinator] Routing action: %s", action)
			// Dispatches to LiveSplit server...

		case <-ticker.C:
			// Regular tick: evaluate active condition if connected
		}
	}
}

func (c *Coordinator) EmitAction(action SplitAction) {
	select {
	case c.actions <- action:
	default:
		log.Printf("[Coordinator] Warning: Action queue full, dropped: %s", action)
	}
}
