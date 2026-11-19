// Package coordinator implements the runtime that coordinates the LiveSplit One server, the SNI bridge, and the split evaluation engine.
package coordinator

import (
	"context"
	"fmt"
	"log"
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

	operationMu        sync.Mutex
	mu                 sync.Mutex
	runStarted         bool
	splitIndex         int
	currentSplitName   string
	unmatchedSplitName string
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
		c.server.SetSplitCallback(func(status livesplit.SplitStatus) {
			if status.Running {
				c.mu.Lock()
				c.currentSplitName = status.CurrentSplitName
				c.unmatchedSplitName = ""
				c.mu.Unlock()
				log.Printf("LiveSplit One current split: %q", status.CurrentSplitName)
				return
			}
			if c.hasActiveRun() {
				c.resetRunState()
			}
		})
		c.server.SetErrorCallback(func(err error) {
			log.Printf("LiveSplit One protocol: %v", err)
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
	folder := c.cfg.GameDefinitionFolder
	if folder == "" {
		defaultFolder, err := config.DefaultGameDefinitionFolder()
		if err != nil {
			return err
		}
		folder = defaultFolder
	}
	names, err := config.ListGameDefinitions(folder)
	if err != nil {
		return fmt.Errorf("list game definitions: %w", err)
	}
	if len(names) == 0 {
		return fmt.Errorf("no viable game definition: no JSON definition files found in %s", folder)
	}
	selectedDefinitionExists := false
	for _, name := range names {
		if name == c.cfg.CurrentGameDefinition {
			selectedDefinitionExists = true
			break
		}
	}
	if !selectedDefinitionExists {
		c.cfg.CurrentGameDefinition = ""
	}

	path, filename, err := config.ResolveGameDefinition(folder, c.cfg.CurrentGameDefinition)
	if err != nil {
		return fmt.Errorf("resolve game definition: %w", err)
	}
	def, err := engine.LoadDefinition(path)
	if err != nil {
		return fmt.Errorf("definition file %s: %w", path, err)
	}

	selectionChanged := c.cfg.GameDefinitionFolder != folder || c.cfg.CurrentGameDefinition != filename
	c.mu.Lock()
	c.definition = def
	c.cfg.GameDefinitionFolder = folder
	c.cfg.CurrentGameDefinition = filename
	c.mu.Unlock()
	if selectionChanged {
		if err := config.Save(c.cfg); err != nil {
			return fmt.Errorf("save selected game definition: %w", err)
		}
	}
	return nil
}

// SetGameDefinition updates the active definition and persists the selection.
// An empty filename clears the active definition while retaining the folder.
func (c *Coordinator) SetGameDefinition(folder, filename string) error {
	c.operationMu.Lock()
	defer c.operationMu.Unlock()

	if folder == "" {
		defaultFolder, err := config.DefaultGameDefinitionFolder()
		if err != nil {
			return err
		}
		folder = defaultFolder
	}

	var def *engine.GameDefinition
	if filename != "" {
		path, selected, err := config.ResolveGameDefinition(folder, filename)
		if err != nil {
			return fmt.Errorf("resolve game definition: %w", err)
		}
		def, err = engine.LoadDefinition(path)
		if err != nil {
			return fmt.Errorf("definition file %s: %w", path, err)
		}
		filename = selected
	}

	if c.cfg == nil {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load configuration: %w", err)
		}
		c.cfg = cfg
	}
	nextConfig := *c.cfg
	nextConfig.GameDefinitionFolder = folder
	nextConfig.CurrentGameDefinition = filename
	if err := config.Save(&nextConfig); err != nil {
		return fmt.Errorf("save game definition selection: %w", err)
	}

	c.mu.Lock()
	c.cfg.GameDefinitionFolder = folder
	c.cfg.CurrentGameDefinition = filename
	c.definition = def
	c.runStarted = false
	c.splitIndex = 0
	c.currentSplitName = ""
	c.unmatchedSplitName = ""
	c.trackers.Reset()
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
	c.operationMu.Lock()
	defer c.operationMu.Unlock()

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
	if c.server.ConnectedClients() == 0 {
		return nil
	}

	c.mu.Lock()
	def := c.definition
	tracker := c.trackers
	runStarted := c.runStarted
	splitIndex := c.splitIndex
	currentSplitName := c.currentSplitName
	c.mu.Unlock()
	if def == nil {
		return nil
	}
	if !runStarted {
		if def.Autostart == nil {
			c.mu.Lock()
			c.runStarted = true
			c.currentSplitName = ""
			c.mu.Unlock()
			return nil
		}
		matched, err := tracker.Evaluate(def.Autostart, c.sniClient)
		if err != nil {
			return err
		}
		if matched {
			if err := c.dispatchAction("start"); err != nil {
				return err
			}
			c.mu.Lock()
			c.runStarted = true
			c.currentSplitName = ""
			c.mu.Unlock()
		}
		return nil
	}

	if c.cfg.EventFile == "" {
		if currentSplitName == "" {
			if c.server.ConnectedClients() == 0 {
				return nil
			}
			return c.server.RequestCurrentSplitName()
		}
		splitIndex = -1
		for i := range def.Definitions {
			if def.Definitions[i].Name == currentSplitName {
				splitIndex = i
				break
			}
		}
		if splitIndex < 0 {
			c.mu.Lock()
			shouldLog := c.unmatchedSplitName != currentSplitName
			c.unmatchedSplitName = currentSplitName
			c.mu.Unlock()
			if shouldLog {
				log.Printf("LiveSplit current split %q has no matching definition", currentSplitName)
			}
			if err := c.dispatchAction("skip"); err != nil {
				return err
			}
			c.mu.Lock()
			c.currentSplitName = ""
			c.unmatchedSplitName = ""
			c.mu.Unlock()
			return nil
		}
		c.mu.Lock()
		c.unmatchedSplitName = ""
		c.mu.Unlock()
	} else if splitIndex >= len(def.Definitions) {
		return nil
	}
	matched, err := engine.Evaluate(&def.Definitions[splitIndex], c.sniClient)
	if err != nil {
		return err
	}
	if matched {
		if err := c.dispatchAction("split"); err != nil {
			return err
		}
		c.mu.Lock()
		c.splitIndex++
		c.currentSplitName = ""
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
	c.currentSplitName = ""
	c.unmatchedSplitName = ""
	if c.trackers != nil {
		c.trackers.Reset()
	}
}

func (c *Coordinator) dispatchAction(action string) error {
	if c.server == nil || action == "" {
		return nil
	}
	if action == "reset" {
		return fmt.Errorf("reset requires an explicit attempt-history decision")
	}
	if err := c.server.DispatchAction(action); err != nil {
		return err
	}
	log.Printf("Dispatching LiveSplit action: %s", action)
	return nil
}

// ResetTimer asks LiveSplit One to reset with an explicit attempt-history choice.
func (c *Coordinator) ResetTimer(saveAttempt bool) error {
	return c.ResetTimerWithOptions(saveAttempt, false)
}

// ResetTimerWithOptions resets LiveSplit and optionally asks SNI to reset the SNES.
func (c *Coordinator) ResetTimerWithOptions(saveAttempt, resetSNES bool) error {
	c.operationMu.Lock()
	defer c.operationMu.Unlock()

	if c.server == nil {
		return fmt.Errorf("LiveSplit server is not running")
	}
	if err := c.server.ResetAttempt(saveAttempt); err != nil {
		return err
	}
	c.resetRunState()
	log.Printf("Reset LiveSplit timer (save attempt=%t)", saveAttempt)

	if resetSNES {
		if c.cfg == nil || c.cfg.SNIAddress == "" {
			return fmt.Errorf("reset SNES game: SNI address is not configured")
		}
		if c.sniClient == nil {
			c.sniClient = sni.NewClient(c.cfg.SNIAddress)
		}
		if !c.sniClient.IsConnected() {
			if err := c.sniClient.Connect(); err != nil {
				return fmt.Errorf("reset SNES game: connect to SNI: %w", err)
			}
		}
		if err := c.sniClient.ResetGame(); err != nil {
			return fmt.Errorf("reset SNES game: %w", err)
		}
		log.Printf("Sent SNES reset request via SNI")
	}
	return nil
}
