package test

import (
	"path/filepath"
	"testing"
	"time"

	"goto_splitter/engine"
	"goto_splitter/livesplit"
	"goto_splitter/sni"
)

func TestMockServersInitialization(t *testing.T) {
	// 1. Start Mock SNI server on an ephemeral port
	sniMock := NewMockSNI("0")
	if err := sniMock.Start(); err != nil {
		t.Fatalf("Failed to start mock SNI server: %v", err)
	}
	defer sniMock.Stop()

	// 2. Start GotoSplitter LiveSplit One server on an ephemeral port
	lsServer := livesplit.NewServer("127.0.0.1:0", nil)
	if err := lsServer.Start(); err != nil {
		t.Fatalf("Failed to start LiveSplit server: %v", err)
	}
	defer lsServer.Stop()

	// 3. Connect Mock LiveSplit client to LiveSplit server
	lsMock := NewMockLiveSplit("")
	if err := lsMock.Connect(lsServer.URL()); err != nil {
		t.Fatalf("Failed to connect mock LiveSplit client: %v", err)
	}
	defer lsMock.Stop()

	// Give connection time to establish
	time.Sleep(100 * time.Millisecond)

	if lsServer.ConnectedClients() != 1 {
		t.Errorf("Expected 1 connected LiveSplit client, got %d", lsServer.ConnectedClients())
	}

	// 4. Connect SNI client to mock SNI server
	sniClient := sni.NewClient(sniMock.URL())
	if err := sniClient.Connect(); err != nil {
		t.Fatalf("Failed to connect SNI client to mock SNI server: %v", err)
	}
	defer sniClient.Close()

	if !sniClient.IsConnected() {
		t.Fatal("Expected SNI client to report connected")
	}

	// 5. Dispatch action and verify immediate clearing
	lsServer.DispatchAction("start")

	if err := lsMock.WaitForAction("starttimer", 1*time.Second); err != nil {
		t.Fatalf("LiveSplit client did not receive start action: %v", err)
	}

	if err := lsMock.VerifyImmediateClearing(); err != nil {
		t.Fatalf("Immediate clearing verification failed: %v", err)
	}
}

func TestLiveSplitMetadataAndQueries(t *testing.T) {
	metadataUpdates := make(chan livesplit.RunMetadata, 1)
	lsServer := livesplit.NewServer("127.0.0.1:0", nil)
	lsServer.SetMetadataCallback(func(metadata livesplit.RunMetadata) {
		metadataUpdates <- metadata
	})
	if err := lsServer.Start(); err != nil {
		t.Fatalf("Failed to start LiveSplit server: %v", err)
	}
	defer lsServer.Stop()

	lsMock := NewMockLiveSplit("")
	if err := lsMock.Connect(lsServer.URL()); err != nil {
		t.Fatalf("Failed to connect mock LiveSplit client: %v", err)
	}
	defer lsMock.Stop()

	if err := lsMock.SendGameInfo("Super Metroid", "Any%"); err != nil {
		t.Fatalf("Failed to send run metadata: %v", err)
	}
	select {
	case metadata := <-metadataUpdates:
		if metadata.GameName != "Super Metroid" || metadata.CategoryName != "Any%" {
			t.Fatalf("Unexpected metadata: %+v", metadata)
		}
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for run metadata")
	}

	if err := lsMock.SendQuery("getGameName"); err != nil {
		t.Fatalf("Failed to query game name: %v", err)
	}
	if err := lsMock.WaitForAction("Super Metroid", time.Second); err != nil {
		t.Fatalf("LiveSplit server did not reply with the game name: %v", err)
	}
}

func TestSuperMetroidFullIntegrationWithMocks(t *testing.T) {
	// 1. Start Mock SNI server
	sniMock := NewMockSNI("0")
	if err := sniMock.Start(); err != nil {
		t.Fatalf("Failed to start mock SNI server: %v", err)
	}
	defer sniMock.Stop()

	// 2. Start LiveSplit server
	lsServer := livesplit.NewServer("127.0.0.1:0", nil)
	if err := lsServer.Start(); err != nil {
		t.Fatalf("Failed to start LiveSplit server: %v", err)
	}
	defer lsServer.Stop()

	// 3. Connect Mock LiveSplit client
	lsMock := NewMockLiveSplit("")
	if err := lsMock.Connect(lsServer.URL()); err != nil {
		t.Fatalf("Failed to connect mock LiveSplit client: %v", err)
	}
	defer lsMock.Stop()

	// 4. Connect SNI client to Mock SNI
	sniClient := sni.NewClient(sniMock.URL())
	if err := sniClient.Connect(); err != nil {
		t.Fatalf("Failed to connect SNI client: %v", err)
	}
	defer sniClient.Close()

	time.Sleep(100 * time.Millisecond)

	// 5. Load Super Metroid definition
	defPath := filepath.Join("..", "config", "definitions", "super_metroid.json")
	def, err := engine.LoadDefinition(defPath)
	if err != nil {
		t.Fatalf("Failed to load Super Metroid definition: %v", err)
	}

	// 6. Test Autostart sequence across SNI bridge
	autostartTracker := engine.NewNextTracker()

	// Check on zero memory
	matched, err := autostartTracker.Evaluate(def.Autostart, sniClient)
	if err != nil || matched {
		t.Fatalf("Autostart triggered unexpectedly on empty memory: %v", err)
	}

	// Step 1: Set Game State (0x0998) to 0x02 in MockSNI
	sniMock.SetByte(0x0998, 0x02)
	matched, err = autostartTracker.Evaluate(def.Autostart, sniClient)
	if err != nil || matched {
		t.Fatalf("Autostart triggered prematurely on step 1: %v", err)
	}
	if autostartTracker.Step() != 1 {
		t.Fatalf("Expected tracker step 1, got %d", autostartTracker.Step())
	}

	// Step 2: Set Ceres state (0x0DE2) to 0x0C in MockSNI
	sniMock.SetByte(0x0DE2, 0x0C)
	matched, err = autostartTracker.Evaluate(def.Autostart, sniClient)
	if err != nil || !matched {
		t.Fatalf("Autostart failed to trigger on step 2: %v", err)
	}

	// Dispatch Start to LiveSplit One
	lsServer.DispatchAction("start")

	if err := lsMock.WaitForAction("starttimer", 1*time.Second); err != nil {
		t.Fatalf("Timed out waiting for 'starttimer' action: %v", err)
	}

	// 7. Test Split 1: Landing Site (0x079B == 0x91F8, weq)
	landingSite := findDefByName(def.Definitions, "Landing Site")
	if landingSite == nil {
		t.Fatal("Landing Site split not found")
	}

	// Before memory update
	matched, err = engine.Evaluate(landingSite, sniClient)
	if err != nil || matched {
		t.Fatalf("Landing Site matched before memory set: %v", err)
	}

	// Set 0x079B to 0x91F8
	sniMock.SetWord(0x079B, 0x91F8)
	matched, err = engine.Evaluate(landingSite, sniClient)
	if err != nil || !matched {
		t.Fatalf("Landing Site failed to match: %v", err)
	}

	// Dispatch Split to LiveSplit One
	lsServer.DispatchAction("split")
	if err := lsMock.WaitForAction("split", 1*time.Second); err != nil {
		t.Fatalf("Timed out waiting for Landing Site 'split': %v", err)
	}

	// 8. Test Split 2: Gauntlet Entrance (0x079B == 0x92B3, weq)
	gauntlet := findDefByName(def.Definitions, "Gauntlet Entrance")
	if gauntlet == nil {
		t.Fatal("Gauntlet Entrance split not found")
	}

	sniMock.SetWord(0x079B, 0x92B3)
	matched, err = engine.Evaluate(gauntlet, sniClient)
	if err != nil || !matched {
		t.Fatalf("Gauntlet Entrance failed to match: %v", err)
	}

	lsServer.DispatchAction("split")

	// 9. Test Split 3: Entering Lower Norfair Elevator (0x079B == 0xAF3F AND 0x0E18 == 0x01)
	norfair := findDefByName(def.Definitions, "Entering Lower Norfair Elevator")
	if norfair == nil {
		t.Fatal("Entering Lower Norfair Elevator split not found")
	}

	// Set room ID only
	sniMock.SetWord(0x079B, 0xAF3F)
	sniMock.SetByte(0x0E18, 0x00)
	matched, err = engine.Evaluate(norfair, sniClient)
	if err != nil || matched {
		t.Fatalf("Entering Lower Norfair Elevator matched without 'more' sub-condition: %v", err)
	}

	// Set both room ID and elevator flag
	sniMock.SetByte(0x0E18, 0x01)
	matched, err = engine.Evaluate(norfair, sniClient)
	if err != nil || !matched {
		t.Fatalf("Entering Lower Norfair Elevator failed to match: %v", err)
	}

	lsServer.DispatchAction("split")

	// 10. Verify Immediate Clearing across the entire sequence
	time.Sleep(100 * time.Millisecond)
	if err := lsMock.VerifyImmediateClearing(); err != nil {
		t.Fatalf("Immediate action clearing failed: %v", err)
	}

	msgs := lsMock.GetMessages()
	expectedSequence := []string{"starttimer", "", "split", "", "split", "", "split", ""}
	if len(msgs) != len(expectedSequence) {
		t.Fatalf("Expected %d total messages, got %d: %v", len(expectedSequence), len(msgs), msgs)
	}

	for i, expected := range expectedSequence {
		if msgs[i] != expected {
			t.Errorf("Message index %d: expected %q, got %q", i, expected, msgs[i])
		}
	}
}

func TestMockSNIAddressNormalizationOverSocket(t *testing.T) {
	sniMock := NewMockSNI("0")
	if err := sniMock.Start(); err != nil {
		t.Fatalf("Failed to start mock SNI: %v", err)
	}
	defer sniMock.Stop()

	sniClient := sni.NewClient(sniMock.URL())
	if err := sniClient.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer sniClient.Close()

	// Write test byte to Super Metroid game state address 0x0998
	testVal := byte(0x42)
	sniMock.SetByte(0x0998, testVal)

	// Read via 0x0998 (offset)
	data1, err := sniClient.ReadMemory(0x0998, 1)
	if err != nil || len(data1) == 0 || data1[0] != testVal {
		t.Fatalf("Read via 0x0998 failed: data=%v err=%v", data1, err)
	}

	// Read via 0x7E0998 (SNES WRAM bus address)
	data2, err := sniClient.ReadMemory(0x7E0998, 1)
	if err != nil || len(data2) == 0 || data2[0] != testVal {
		t.Fatalf("Read via 0x7E0998 failed: data=%v err=%v", data2, err)
	}

	// Read via 0xF50998 (QUsb2Snes raw offset)
	data3, err := sniClient.ReadMemory(0xF50998, 1)
	if err != nil || len(data3) == 0 || data3[0] != testVal {
		t.Fatalf("Read via 0xF50998 failed: data=%v err=%v", data3, err)
	}

	requests := sniMock.GetAddressRequests()
	if len(requests) != 3 {
		t.Fatalf("Expected 3 GetAddress requests, got %d", len(requests))
	}
	for i, request := range requests {
		if request.Address != 0xF50998 {
			t.Errorf("GetAddress request %d used address 0x%X, want canonical WRAM address 0xF50998", i, request.Address)
		}
	}
}
