package coordinator

import (
	"context"
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
		LiveSplitAddress: "127.0.0.1:0",
		GameDefinition:   "missing-definition.json",
	}

	c := NewCoordinator(cfg)
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("expected start to fail on invalid game definition")
	}
	if c.ServerURL() != "" {
		t.Fatalf("expected no server URL before a successful start, got %q", c.ServerURL())
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
