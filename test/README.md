# Test Infrastructure for GotoSplitter

This directory contains the mock servers and test harnesses used to verify the functionality of GotoSplitter without requiring physical hardware or real emulators.

## Mock Servers

### Mock SNI Server (`mock_sni.go`)

The mock SNI server simulates a QUsb2Snes/SNI compatible device for testing purposes. It supports:

- DeviceList: Returns a list of available devices
- Attach: Handles device attachment requests  
- GetAddress: Simulates memory reads from specified addresses

### Mock LiveSplit Client (`mock_livesplit.go`)

The mock LiveSplit client simulates the LiveSplit One WebSocket client and provides:

- Connection to GotoSplitter's WebSocket server
- Message recording for verification
- Immediate action clearing verification (ensures that every non-empty action is immediately followed by an empty frame)
- Game information simulation

## Test Suite Structure

### `e2e_test.go`
End-to-end integration tests that verify:
- Initializing Mock SNI and LiveSplit One WebSocket servers
- Full integration with Super Metroid definitions (`super_metroid.json`)
- Address normalization over the WebSocket bridge (`0x0998`, `0x7E0998`, and `0xF50998` map to same memory)
- Autostart multi-step chronological progression
- Split conditions triggering actions (`start`, `split`)
- Immediate action clearing verification (action frame followed immediately by an empty frame `""`)

### `parser_test.go` 
Comprehensive tests for definition schema parsing and condition evaluation:
- Complete parsing of `config/definitions/super_metroid.json` (397 split definitions, autostart, IGT, aliases)
- Condition type evaluation (`weq`, `eq`, `bit`, `wgt`, `more`, `next`)
- Evaluator verification against mock memory

### `sni_hardware_test.go`
Optional, read-only probe for a connected Super Metroid device. It checks memory-read sizes, reports the attached device and selected WRAM values, and verifies that WRAM aliases resolve consistently. QUsb2Snes expects WRAM requests in the `F5xxxx` address range.

Run it explicitly with:

```bash
GOTOSPLITTER_SNI_ADDRESS=ws://127.0.0.1:23074 go test -v ./test -run TestSuperMetroidHardwareMemoryRead
```

## Usage

The mock servers are designed to be used in integration tests where you want to:

1. Test the interaction between GotoSplitter and the SNI server without needing a real device
2. Verify that LiveSplit actions are correctly handled and cleared
3. Validate definition file parsing and condition evaluation
4. Simulate edge cases like connection drops, rapid transitions, etc.

## Running Tests

```bash
go test -v -race ./test/...
```