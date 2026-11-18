package test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goto_splitter/config"
	"goto_splitter/coordinator"
)

func TestCoordinatorStartsAutostartAndSplitSequence(t *testing.T) {
	sniMock := NewMockSNI("0")
	if err := sniMock.Start(); err != nil {
		t.Fatalf("start mock SNI: %v", err)
	}
	defer sniMock.Stop()

	defPath := filepath.Join("..", "config", "definitions", "super_metroid.json")
	cfg := &config.Config{
		SNIAddress:            sniMock.URL(),
		LiveSplitAddress:      "127.0.0.1:0",
		GameDefinitionFolder:  filepath.Dir(defPath),
		CurrentGameDefinition: filepath.Base(defPath),
	}

	c := coordinator.NewCoordinator(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start coordinator: %v", err)
	}
	defer c.Stop()

	lsMock := NewMockLiveSplit("")
	lsMock.SetCurrentSplitName("Landing Site")
	if err := lsMock.Connect(c.ServerURL()); err != nil {
		t.Fatalf("connect mock LiveSplit: %v", err)
	}
	defer lsMock.Stop()

	// Set autostart sequence for Super Metroid.
	sniMock.SetByte(0x0998, 0x02)
	sniMock.SetByte(0x0DE2, 0x0C)

	if err := lsMock.WaitForAction("start", 2*time.Second); err != nil {
		t.Fatalf("did not receive start action: %v", err)
	}

	// Set the first split condition: Landing Site at 0x079B = 0x91F8.
	sniMock.SetWord(0x079B, 0x91F8)
	if err := lsMock.WaitForAction("split", 2*time.Second); err != nil {
		t.Fatalf("did not receive split action: %v", err)
	}
}

func TestCoordinatorResetCanAlsoResetSNES(t *testing.T) {
	sniMock := NewMockSNI("0")
	if err := sniMock.Start(); err != nil {
		t.Fatalf("start mock SNI: %v", err)
	}
	defer sniMock.Stop()

	defPath := filepath.Join("..", "config", "definitions", "super_metroid.json")
	cfg := &config.Config{
		SNIAddress:            sniMock.URL(),
		LiveSplitAddress:      "127.0.0.1:0",
		GameDefinitionFolder:  filepath.Dir(defPath),
		CurrentGameDefinition: filepath.Base(defPath),
	}
	c := coordinator.NewCoordinator(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start coordinator: %v", err)
	}
	defer c.Stop()

	lsMock := NewMockLiveSplit("")
	if err := lsMock.Connect(c.ServerURL()); err != nil {
		t.Fatalf("connect mock LiveSplit: %v", err)
	}
	defer lsMock.Stop()

	if err := c.ResetTimerWithOptions(false, false); err != nil {
		t.Fatalf("reset timer without SNES reset: %v", err)
	}
	if err := lsMock.WaitForAction("reset", time.Second); err != nil {
		t.Fatalf("LiveSplit did not receive reset: %v", err)
	}
	if got := sniMock.ResetRequests(); got != 0 {
		t.Fatalf("SNI reset requests = %d, want 0 when SNES reset is not selected", got)
	}

	if err := c.ResetTimerWithOptions(true, true); err != nil {
		t.Fatalf("reset timer and SNES: %v", err)
	}
	if err := lsMock.WaitForAction("reset", time.Second); err != nil {
		t.Fatalf("LiveSplit did not receive combined reset: %v", err)
	}
	if saveAttempt := lsMock.LastSaveAttempt(); saveAttempt == nil || !*saveAttempt {
		t.Fatalf("save attempt choice = %v, want true", saveAttempt)
	}
	if err := sniMock.WaitForResetRequest(time.Second); err != nil {
		t.Fatal(err)
	}
	if got := sniMock.ResetRequests(); got != 1 {
		t.Fatalf("SNI reset requests = %d, want 1", got)
	}
}

func TestCoordinatorReportsSNIFailureAfterResettingTimer(t *testing.T) {
	defPath := filepath.Join("..", "config", "definitions", "super_metroid.json")
	cfg := &config.Config{
		SNIAddress:            "ws://127.0.0.1:1",
		LiveSplitAddress:      "127.0.0.1:0",
		GameDefinitionFolder:  filepath.Dir(defPath),
		CurrentGameDefinition: filepath.Base(defPath),
	}
	c := coordinator.NewCoordinator(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start coordinator: %v", err)
	}
	defer c.Stop()

	lsMock := NewMockLiveSplit("")
	if err := lsMock.Connect(c.ServerURL()); err != nil {
		t.Fatalf("connect mock LiveSplit: %v", err)
	}
	defer lsMock.Stop()

	err := c.ResetTimerWithOptions(false, true)
	if err == nil || !strings.Contains(err.Error(), "reset SNES game") {
		t.Fatalf("reset error = %v, want SNI reset failure", err)
	}
	if err := lsMock.WaitForAction("reset", time.Second); err != nil {
		t.Fatalf("LiveSplit did not receive reset despite SNI failure: %v", err)
	}
	if saveAttempt := lsMock.LastSaveAttempt(); saveAttempt == nil || *saveAttempt {
		t.Fatalf("save attempt choice = %v, want false", saveAttempt)
	}
}
