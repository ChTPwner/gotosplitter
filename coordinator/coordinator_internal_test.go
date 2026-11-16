package coordinator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goto_splitter/config"
	"goto_splitter/engine"
)

type staticMemoryReader struct{ data []byte }

func (r staticMemoryReader) ReadMemory(address uint32, size int) ([]byte, error) {
	start := int(address)
	if start+size > len(r.data) {
		return nil, nil
	}
	return r.data[start : start+size], nil
}

func TestStartFailsBeforeStartingLiveSplitServer(t *testing.T) {
	cfg := &config.Config{
		LiveSplitAddress:      "127.0.0.1:0",
		GameDefinitionFolder:  t.TempDir(),
		CurrentGameDefinition: "missing-definition.json",
	}

	c := NewCoordinator(cfg)
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("expected start to fail on invalid game definition")
	}
	if c.ServerURL() != "" {
		t.Fatalf("expected no server URL before a successful start, got %q", c.ServerURL())
	}
}

func TestLoadDefinitionSelectsFirstAlphabeticalFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	folder := t.TempDir()
	for name, game := range map[string]string{
		"zeta.json":  "Zeta",
		"alpha.json": "Alpha",
	} {
		data := []byte(`{"name":"` + game + `","definitions":[]}`)
		if err := os.WriteFile(filepath.Join(folder, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{GameDefinitionFolder: folder}
	c := NewCoordinator(cfg)
	if err := c.loadDefinition(); err != nil {
		t.Fatalf("loadDefinition() error = %v", err)
	}
	if cfg.CurrentGameDefinition != "alpha.json" {
		t.Fatalf("CurrentGameDefinition = %q, want alpha.json", cfg.CurrentGameDefinition)
	}
	if c.definition == nil || c.definition.Name != "Alpha" {
		t.Fatalf("loaded definition = %#v, want Alpha", c.definition)
	}
}

func TestLoadDefinitionRejectsAnEmptyFolder(t *testing.T) {
	folder := t.TempDir()
	cfg := &config.Config{GameDefinitionFolder: folder}
	c := NewCoordinator(cfg)
	err := c.loadDefinition()
	if err == nil {
		t.Fatal("expected an error when the definition folder has no JSON files")
	}
	if !strings.Contains(err.Error(), folder) {
		t.Fatalf("loadDefinition() error = %v, want folder path %q", err, folder)
	}
	if !strings.Contains(err.Error(), "no viable game definition") {
		t.Fatalf("loadDefinition() error = %v, want viable-definition error", err)
	}
}

func TestStartFailsWhenDefinitionFolderHasNoViableFile(t *testing.T) {
	cfg := &config.Config{
		LiveSplitAddress:     "127.0.0.1:0",
		GameDefinitionFolder: t.TempDir(),
	}
	c := NewCoordinator(cfg)
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("expected Start() to fail without a viable definition")
	}
	if c.ServerURL() != "" {
		t.Fatalf("expected no LiveSplit server URL, got %q", c.ServerURL())
	}
}

func TestLoadDefinitionRejectsUnparseableDefinition(t *testing.T) {
	folder := t.TempDir()
	if err := os.WriteFile(filepath.Join(folder, "invalid.json"), []byte(`{"name":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		GameDefinitionFolder:  folder,
		CurrentGameDefinition: "invalid.json",
	}
	c := NewCoordinator(cfg)
	if err := c.loadDefinition(); err == nil {
		t.Fatal("expected an error for an invalid game definition")
	}
}

func TestSetGameDefinitionPersistsAndAppliesSelection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	folder := t.TempDir()
	definition := filepath.Join(folder, "game.json")
	if err := os.WriteFile(definition, []byte(`{"name":"Test Game","definitions":[]}`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	c := NewCoordinator(cfg)
	if err := c.SetGameDefinition(folder, "game.json"); err != nil {
		t.Fatalf("SetGameDefinition() error = %v", err)
	}
	if c.definition == nil || c.definition.Name != "Test Game" {
		t.Fatalf("active definition = %#v, want Test Game", c.definition)
	}
	if cfg.GameDefinitionFolder != folder || cfg.CurrentGameDefinition != "game.json" {
		t.Fatalf("config selection = (%q, %q), want (%q, game.json)", cfg.GameDefinitionFolder, cfg.CurrentGameDefinition, folder)
	}
	persisted, err := config.Load()
	if err != nil {
		t.Fatalf("Load() persisted config error = %v", err)
	}
	if persisted.GameDefinitionFolder != folder || persisted.CurrentGameDefinition != "game.json" {
		t.Fatalf("persisted selection = (%q, %q), want (%q, game.json)", persisted.GameDefinitionFolder, persisted.CurrentGameDefinition, folder)
	}
}

func TestResetRunStateClearsTracking(t *testing.T) {
	trackers := engine.NewNextTracker()
	cond := &engine.Condition{
		Address: "0x0",
		Value:   "0x1",
		Type:    engine.TypeEq,
		Next: []engine.Condition{{
			Address: "0x1",
			Value:   "0x2",
			Type:    engine.TypeEq,
		}},
	}
	if err := engine.ParseAndNormalizeCondition(cond); err != nil {
		t.Fatalf("parse condition: %v", err)
	}
	if _, err := trackers.Evaluate(cond, staticMemoryReader{data: []byte{0x1}}); err != nil {
		t.Fatalf("evaluate condition: %v", err)
	}
	if trackers.Step() == 0 {
		t.Fatal("expected tracker to advance before reset")
	}

	c := &Coordinator{trackers: trackers}
	c.runStarted = true
	c.splitIndex = 3

	c.resetRunState()
	if c.runStarted {
		t.Fatal("runStarted was not reset")
	}
	if c.splitIndex != 0 {
		t.Fatalf("splitIndex = %d, want 0", c.splitIndex)
	}
	if c.trackers.Step() != 0 {
		t.Fatalf("tracker step = %d, want 0", c.trackers.Step())
	}
}
