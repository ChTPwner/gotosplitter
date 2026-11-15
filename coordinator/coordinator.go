package coordinator

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"goto_splitter/config"
	"goto_splitter/engine"
	"goto_splitter/livesplit"
	"goto_splitter/sni"
)

const defaultPollInterval = 50 * time.Millisecond

// Coordinator is the system runtime that wires together the LiveSplit server,
// the SNI bridge, and the split evaluator.
type Coordinator struct {
	cfg          *config.Config
	server       *livesplit.Server
	sniClient    *sni.SNI
	definition   *engine.GameDefinition
	trackers     *engine.NextTracker
	pollInterval time.Duration

	mu         sync.Mutex
	runStarted bool
	splitIndex int
}

// NewCoordinator builds a coordinator from the supplied configuration.
func NewCoordinator(cfg *config.Config) *Coordinator {
	if cfg == nil {
		var err error
		cfg, err = config.Load()
		if err != nil {
			log.Printf("failed to load configuration: %v", err)
		}
	}
	return &Coordinator{
		cfg:          cfg,
		trackers:     engine.NewNextTracker(),
		pollInterval: defaultPollInterval,
	}
}

// Start boots the coordinator and launches the background runtime loop.
func (c *Coordinator) Start(ctx context.Context) error {
	if c.cfg == nil {
		var err error
		c.cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("load configuration: %w", err)
		}
	}

	if err := c.loadDefinition(); err != nil {
		return fmt.Errorf("load definition: %w", err)
	}

	if c.server == nil {
		c.server = livesplit.NewServer(c.cfg.LiveSplitAddress, func(connected bool, count int) {
			log.Printf("LiveSplit One clients connected: %d (connected=%t)", count, connected)
		})
		c.server.SetMetadataCallback(func(metadata livesplit.RunMetadata) {
			log.Printf("LiveSplit metadata: game=%q category=%q split=%q", metadata.GameName, metadata.CategoryName, metadata.CurrentSplitName)
			if metadata.CurrentSplitName == "" && c.hasActiveRun() {
				c.resetRunState()
			}
		})
	}
	if err := c.server.Start(); err != nil {
		return fmt.Errorf("start LiveSplit server: %w", err)
	}

	if c.cfg.SNIAddress != "" {
		c.sniClient = sni.NewClient(c.cfg.SNIAddress)
		if err := c.sniClient.Connect(); err != nil {
			log.Printf("SNI not connected yet; retrying in polling loop: %v", err)
		} else {
			log.Printf("SNI connected to %s", c.sniClient.AttachedDevice())
		}
	}

	go c.runLoop(ctx)
	return nil
}

// ServerURL returns the WebSocket URL of the LiveSplit server.
func (c *Coordinator) ServerURL() string {
	if c.server == nil {
		return ""
	}
	return c.server.URL()
}

// Stop shuts down the coordinator cleanly.
func (c *Coordinator) Stop() error {
	if c.sniClient != nil {
		if err := c.sniClient.Close(); err != nil {
			log.Printf("warning: could not close SNI client cleanly: %v", err)
		}
	}
	if c.server != nil {
		if err := c.server.Stop(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Coordinator) loadDefinition() error {
	path := c.cfg.GameDefinition
	if path == "" {
		candidates := []string{
			filepath.Join("config", "definitions", "super_metroid.json"),
			filepath.Join("definitions", "super_metroid.json"),
			filepath.Join(".", "config", "definitions", "super_metroid.json"),
			filepath.Join(".", "definitions", "super_metroid.json"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	if path == "" {
		return fmt.Errorf("no definition configured")
	}

	def, err := engine.LoadDefinition(path)
	if err != nil {
		return fmt.Errorf("definition file %s: %w", path, err)
	}

	c.mu.Lock()
	c.definition = def
	c.mu.Unlock()
	return nil
}

func (c *Coordinator) runLoop(ctx context.Context) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.pollOnce(); err != nil {
				log.Printf("coordinator tick: %v", err)
			}
		}
	}
}

func (c *Coordinator) pollOnce() error {
	if c.server == nil {
		return nil
	}
	if c.cfg.SNIAddress == "" {
		return nil
	}
	if c.sniClient == nil {
		c.sniClient = sni.NewClient(c.cfg.SNIAddress)
	}
	if !c.sniClient.IsConnected() {
		if err := c.sniClient.Connect(); err != nil {
			return err
		}
		log.Printf("SNI connected to %s", c.sniClient.AttachedDevice())
	}

	c.mu.Lock()
	def := c.definition
	tracker := c.trackers
	runStarted := c.runStarted
	splitIndex := c.splitIndex
	c.mu.Unlock()
	if def == nil {
		return nil
	}
	if !runStarted {
		if def.Autostart == nil {
			c.mu.Lock()
			c.runStarted = true
			c.mu.Unlock()
			return nil
		}
		matched, err := tracker.Evaluate(def.Autostart, c.sniClient)
		if err != nil {
			return err
		}
		if matched {
			c.mu.Lock()
			c.runStarted = true
			c.mu.Unlock()
			c.dispatchAction("start")
		}
		return nil
	}

	if splitIndex >= len(def.Definitions) {
		return nil
	}
	matched, err := engine.Evaluate(&def.Definitions[splitIndex], c.sniClient)
	if err != nil {
		return err
	}
	if matched {
		c.dispatchAction("split")
		c.mu.Lock()
		c.splitIndex++
		c.mu.Unlock()
	}
	return nil
}

func (c *Coordinator) hasActiveRun() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runStarted || c.splitIndex > 0
}

func (c *Coordinator) resetRunState() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runStarted = false
	c.splitIndex = 0
	if c.trackers != nil {
		c.trackers.Reset()
	}
}

func (c *Coordinator) dispatchAction(action string) {
	if c.server == nil || action == "" {
		return
	}
	if action == "reset" {
		c.resetRunState()
	}
	c.server.DispatchAction(action)
	log.Printf("Dispatching LiveSplit action: %s", action)
}
