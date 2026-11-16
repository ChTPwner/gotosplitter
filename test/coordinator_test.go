package test

import (
	"context"
	"path/filepath"
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
	if err := lsMock.Connect(c.ServerURL()); err != nil {
		t.Fatalf("connect mock LiveSplit: %v", err)
	}
	defer lsMock.Stop()

	// Set autostart sequence for Super Metroid.
	sniMock.SetByte(0x0998, 0x02)
	sniMock.SetByte(0x0DE2, 0x0C)

	if err := lsMock.WaitForAction("starttimer", 2*time.Second); err != nil {
		t.Fatalf("did not receive start action: %v", err)
	}

	// Set the first split condition: Landing Site at 0x079B = 0x91F8.
	sniMock.SetWord(0x079B, 0x91F8)
	if err := lsMock.WaitForAction("split", 2*time.Second); err != nil {
		t.Fatalf("did not receive split action: %v", err)
	}
}
