package test

import (
	"path/filepath"
	"testing"

	"goto_splitter/engine"
)

func TestSuperMetroidDefinitionParsing(t *testing.T) {
	defPath := filepath.Join("..", "config", "definitions", "super_metroid.json")
	def, err := engine.LoadDefinition(defPath)
	if err != nil {
		t.Fatalf("Failed to load Super Metroid definition from %s: %v", defPath, err)
	}

	// 1. Verify Root Fields
	if def.Name != "Super Metroid" {
		t.Errorf("Expected Name 'Super Metroid', got %q", def.Name)
	}

	// 2. Verify Autostart
	if def.Autostart == nil {
		t.Fatal("Expected autostart definition to be present, got nil")
	}
	if def.Autostart.Active != "1" {
		t.Errorf("Expected autostart active='1', got %q", def.Autostart.Active)
	}
	if def.Autostart.ParsedAddress != 0x0998 {
		t.Errorf("Expected autostart address 0x0998, got 0x%X", def.Autostart.ParsedAddress)
	}
	if def.Autostart.ParsedValue != 0x02 {
		t.Errorf("Expected autostart value 0x02, got 0x%X", def.Autostart.ParsedValue)
	}
	if def.Autostart.Type != engine.TypeEq {
		t.Errorf("Expected autostart type 'eq', got %q", def.Autostart.Type)
	}

	// Autostart Next sequence step
	if len(def.Autostart.Next) != 1 {
		t.Fatalf("Expected 1 'next' step in autostart, got %d", len(def.Autostart.Next))
	}
	nextStep := def.Autostart.Next[0]
	if nextStep.ParsedAddress != 0x0DE2 {
		t.Errorf("Expected autostart next step address 0x0DE2, got 0x%X", nextStep.ParsedAddress)
	}
	if nextStep.ParsedValue != 0x0C {
		t.Errorf("Expected autostart next step value 0x0C, got 0x%X", nextStep.ParsedValue)
	}
	if nextStep.Type != engine.TypeEq {
		t.Errorf("Expected autostart next step type 'eq', got %q", nextStep.Type)
	}

	// 3. Verify IGT
	if def.IGT == nil {
		t.Fatal("Expected IGT configuration to be present, got nil")
	}
	if def.IGT.ParsedFramesAddress != 0x09DA {
		t.Errorf("Expected IGT framesAddress 0x09DA, got 0x%X", def.IGT.ParsedFramesAddress)
	}
	if def.IGT.ParsedSecondsAddress != 0x09DC {
		t.Errorf("Expected IGT secondsAddress 0x09DC, got 0x%X", def.IGT.ParsedSecondsAddress)
	}
	if def.IGT.ParsedMinutesAddress != 0x09DE {
		t.Errorf("Expected IGT minutesAddress 0x09DE, got 0x%X", def.IGT.ParsedMinutesAddress)
	}
	if def.IGT.ParsedHoursAddress != 0x09E0 {
		t.Errorf("Expected IGT hoursAddress 0x09E0, got 0x%X", def.IGT.ParsedHoursAddress)
	}

	// 4. Verify Aliases
	if def.Alias == nil {
		t.Fatal("Expected alias map, got nil")
	}
	if def.Alias["G4"] != "Statues Room" {
		t.Errorf("Expected alias 'G4' -> 'Statues Room', got %q", def.Alias["G4"])
	}
	if def.Alias["Ship"] != "Zebes Escape" {
		t.Errorf("Expected alias 'Ship' -> 'Zebes Escape', got %q", def.Alias["Ship"])
	}

	// 5. Verify Split Definitions
	if len(def.Definitions) != 397 {
		t.Errorf("Expected 397 definitions, got %d", len(def.Definitions))
	}

	typeCounts := make(map[engine.ConditionType]int)
	moreCount := 0

	for i, d := range def.Definitions {
		if d.Name == "" {
			t.Errorf("Definition at index %d has empty name", i)
		}
		if d.Address == "" {
			t.Errorf("Definition %q has empty address", d.Name)
		}
		if d.Value == "" {
			t.Errorf("Definition %q has empty value", d.Name)
		}

		typeCounts[d.Type]++
		if len(d.More) > 0 {
			moreCount++
			for j, m := range d.More {
				if m.Address == "" || m.Value == "" {
					t.Errorf("Definition %q 'more' sub-condition %d has empty address or value", d.Name, j)
				}
			}
		}
	}

	t.Logf("Total definitions validated: %d", len(def.Definitions))
	t.Logf("Condition type breakdown: weq=%d, eq=%d, bit=%d, composite with more=%d",
		typeCounts[engine.TypeWeq], typeCounts[engine.TypeEq], typeCounts[engine.TypeBit], moreCount)

	// Validate Landing Site (weq)
	landingSite := findDefByName(def.Definitions, "Landing Site")
	if landingSite == nil {
		t.Fatal("Definition 'Landing Site' not found")
	}
	if landingSite.ParsedAddress != 0x079B || landingSite.ParsedValue != 0x91F8 || landingSite.Type != engine.TypeWeq {
		t.Errorf("Landing Site definition mismatch: addr=0x%X val=0x%X type=%s",
			landingSite.ParsedAddress, landingSite.ParsedValue, landingSite.Type)
	}

	// Validate Hyper Beam (bit)
	hyperBeam := findDefByName(def.Definitions, "Hyper Beam")
	if hyperBeam == nil {
		t.Fatal("Definition 'Hyper Beam' not found")
	}
	if hyperBeam.ParsedAddress != 0x0A77 || hyperBeam.ParsedValue != 0x80 || hyperBeam.Type != engine.TypeBit {
		t.Errorf("Hyper Beam definition mismatch: addr=0x%X val=0x%X type=%s",
			hyperBeam.ParsedAddress, hyperBeam.ParsedValue, hyperBeam.Type)
	}

	// Validate Entering Lower Norfair Elevator (more)
	norfair := findDefByName(def.Definitions, "Entering Lower Norfair Elevator")
	if norfair == nil {
		t.Fatal("Definition 'Entering Lower Norfair Elevator' not found")
	}
	if len(norfair.More) != 1 || norfair.More[0].ParsedAddress != 0x0E18 || norfair.More[0].ParsedValue != 0x01 {
		t.Errorf("Entering Lower Norfair Elevator 'more' condition mismatch")
	}
}

func TestAllBundledDefinitionsParse(t *testing.T) {
	definitions, err := filepath.Glob(filepath.Join("..", "config", "definitions", "*.json"))
	if err != nil {
		t.Fatalf("find bundled definitions: %v", err)
	}
	if len(definitions) == 0 {
		t.Fatal("no bundled definitions found")
	}
	for _, path := range definitions {
		if _, err := engine.LoadDefinition(path); err != nil {
			t.Errorf("load bundled definition %s: %v", path, err)
		}
	}
}

func TestSuperMetroidConditionEvaluationWithMockMemory(t *testing.T) {
	mockSNI := NewMockSNI("0")
	defer mockSNI.Stop()

	defPath := filepath.Join("..", "config", "definitions", "super_metroid.json")
	def, err := engine.LoadDefinition(defPath)
	if err != nil {
		t.Fatalf("Failed to load Super Metroid definition: %v", err)
	}

	// 1. Test Autostart sequence with NextTracker
	tracker := engine.NewNextTracker()

	// Initial memory 0: should not trigger
	mockSNI.ResetMemory()
	matched, err := tracker.Evaluate(def.Autostart, mockSNI)
	if err != nil || matched {
		t.Fatalf("Autostart triggered unexpectedly on empty memory: matched=%v err=%v", matched, err)
	}
	if tracker.Step() != 0 {
		t.Errorf("Expected step 0, got %d", tracker.Step())
	}

	// Step 1: Set 0x0998 to 0x02
	mockSNI.SetByte(0x0998, 0x02)
	matched, err = tracker.Evaluate(def.Autostart, mockSNI)
	if err != nil || matched {
		t.Fatalf("Autostart completed prematurely on step 1: matched=%v err=%v", matched, err)
	}
	if tracker.Step() != 1 {
		t.Fatalf("Expected tracker to advance to step 1, got %d", tracker.Step())
	}

	// Step 2: Set 0x0DE2 to 0x0C
	mockSNI.SetByte(0x0DE2, 0x0C)
	matched, err = tracker.Evaluate(def.Autostart, mockSNI)
	if err != nil {
		t.Fatalf("Autostart evaluation error: %v", err)
	}
	if !matched {
		t.Fatal("Expected autostart to trigger after step 2, but it did not")
	}

	// 2. Test Word Equals (weq) - Landing Site (0x079B == 0x91F8)
	landingSite := findDefByName(def.Definitions, "Landing Site")
	mockSNI.ResetMemory()

	matched, _ = engine.Evaluate(landingSite, mockSNI)
	if matched {
		t.Fatal("Landing Site matched before setting memory")
	}

	// Set 0x079B to 0x91F8 (little endian: byte 0x98 at 0x079B, byte 0x91 at 0x079C)
	mockSNI.SetWord(0x079B, 0x91F8)
	matched, err = engine.Evaluate(landingSite, mockSNI)
	if err != nil || !matched {
		t.Fatalf("Landing Site evaluation failed: matched=%v err=%v", matched, err)
	}

	// 3. Test Bitmask (bit) - Hyper Beam (0x0A77 & 0x80 != 0)
	hyperBeam := findDefByName(def.Definitions, "Hyper Beam")
	mockSNI.ResetMemory()

	mockSNI.SetByte(0x0A77, 0x7F) // Bit 0x80 not set
	matched, _ = engine.Evaluate(hyperBeam, mockSNI)
	if matched {
		t.Fatal("Hyper Beam matched with bit 0x80 clear")
	}

	mockSNI.SetByte(0x0A77, 0xFF) // Bit 0x80 set
	matched, err = engine.Evaluate(hyperBeam, mockSNI)
	if err != nil || !matched {
		t.Fatalf("Hyper Beam evaluation failed: matched=%v err=%v", matched, err)
	}

	// 4. Test Conjunction (more) - Entering Lower Norfair Elevator (0x079B == 0xAF3F AND 0x0E18 == 0x01)
	norfair := findDefByName(def.Definitions, "Entering Lower Norfair Elevator")
	mockSNI.ResetMemory()

	// Only base address set
	mockSNI.SetWord(0x079B, 0xAF3F)
	matched, _ = engine.Evaluate(norfair, mockSNI)
	if matched {
		t.Fatal("Entering Lower Norfair Elevator matched without 'more' condition")
	}

	// Both base and 'more' sub-condition set
	mockSNI.SetByte(0x0E18, 0x01)
	matched, err = engine.Evaluate(norfair, mockSNI)
	if err != nil || !matched {
		t.Fatalf("Entering Lower Norfair Elevator evaluation failed: matched=%v err=%v", matched, err)
	}

	// 5. Test Word Greater Than (wgt) with Conjunction (more) - Mother Brain 2
	// Room 0x079B == 0xDD58 AND HP 0x0FCC > 0x7530 (30,000)
	motherBrain2 := findDefByName(def.Definitions, "Mother Brain 2")
	if motherBrain2 == nil {
		t.Fatal("Mother Brain 2 definition not found")
	}
	mockSNI.ResetMemory()

	// Set correct room 0xDD58, but HP <= 0x7530 (e.g. 0x7530)
	mockSNI.SetWord(0x079B, 0xDD58)
	mockSNI.SetWord(0x0FCC, 0x7530)
	matched, err = engine.Evaluate(motherBrain2, mockSNI)
	if err != nil || matched {
		t.Fatalf("Mother Brain 2 matched with HP equal to threshold (wgt requires strictly greater): %v", err)
	}

	// Set HP > 0x7530 (e.g. 0x7531)
	mockSNI.SetWord(0x0FCC, 0x7531)
	matched, err = engine.Evaluate(motherBrain2, mockSNI)
	if err != nil || !matched {
		t.Fatalf("Mother Brain 2 evaluation failed with HP > threshold: matched=%v err=%v", matched, err)
	}
}

func findDefByName(defs []engine.Condition, name string) *engine.Condition {
	for i := range defs {
		if defs[i].Name == name {
			return &defs[i]
		}
	}
	return nil
}
