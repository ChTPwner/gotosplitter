---
name: qa-harness
description: >-
  Creates mock servers, automated integration harnesses, and verification test suites for GotoSplitter.
  Use this skill when building mock QUsb2Snes/SNI servers, mock LiveSplit One clients, validating definition
  JSON schemas, or verifying rapid split transitions and zero memory leaks in test/.
---

# Test Harness & Verification Skill (`@qa-agent`)

This skill defines the testing strategies, mock server architectures, edge case simulations, and continuous verification procedures for GotoSplitter, owned by `@qa-agent`.

---

## 1. Role & Architectural Boundaries

- **Owning Agent:** `@qa-agent`
- **Primary Package:** `test/`
- **Key Files:** `test/mock_sni.go`, `test/mock_livesplit.go`, `test/e2e_test.go`, `test/parser_test.go`
- **Core Mission:** Ensure reliability, zero memory leaks, and protocol adherence without requiring physical Super Nintendo hardware or real emulators.

```mermaid
graph TD
    subgraph Mock Infrastructure
        MockSNI["Mock SNI / QUsb2Snes Server (:8080)"]
        MockLS["Mock LiveSplit One Client (:16834)"]
    end

    subgraph System Under Test (SUT)
        GotoSplitter["GotoSplitter (Server + Client + Engine)"]
    end

    subgraph Verification Checks
        CheckAction["Verify Split Actions Received"]
        CheckClearing["Verify Immediate Action Clearing"]
        CheckLeak["Verify Zero Leaks & Low CPU"]
        CheckSchema["Validate Real Game Definitions"]
    end

    GotoSplitter --"Connects as Client"--> MockSNI
    MockLS --"Connects as Client"--> GotoSplitter

    GotoSplitter --> CheckAction
    GotoSplitter --> CheckClearing
    GotoSplitter --> CheckLeak
    GotoSplitter --> CheckSchema
```

---

## 2. Core Responsibilities

1. **Mock QUsb2Snes / SNI Server**:
   - WebSocket server capable of running on `ws://localhost:8080` or ephemeral test ports.
   - Handles `Name`, `DeviceList`, `Attach`, and `GetAddress` opcodes.
   - Simulates in-memory RAM banks for standard games (Super Mario World, ALttP, Super Metroid).
   - Allows scripted memory mutations (e.g. set address `0x7E0100` to `0x14` to trigger autostart, then change level exit byte).

2. **Mock LiveSplit One Client**:
   - WebSocket client that connects to GotoSplitter (`ws://localhost:16834`).
   - Records all received frames.
   - **Verifies Immediate Clearing**: Asserts that every non-empty action (e.g. `"split"`) is immediately followed by an empty frame `""` or reset state.

3. **Definition Schema Verification**:
   - Unit test definition parsers against all sample JSON schemas from `ChTPwner/Usb2Snes-Livesplit-Definitions`.
   - Validate condition types: `eq`, `weq`, `bit`, `more`, `next`.
   - Ensure hex string conversions (`"0x7E0100"`, `"0xA0"`) and address bounds are strictly validated.

4. **Edge Case & Fault Injection Testing**:
   - **Rapid Split Transitions**: Simulate back-to-back level exits to ensure no race conditions or dropped splits.
   - **Socket Disconnects**: Abruptly close SNI or LiveSplit connections; verify that GotoSplitter does not crash and reconnects cleanly.
   - **Emulator Pause**: Simulate frozen memory states without spinlocks.
   - **Malformed JSON**: Provide broken definition and event files to verify graceful user error reporting.
   - **Data Race & Leak Checks**: Execute test suites under `go test -race` and profile memory allocations.

---

## 3. Test Suites & Procedures

See test matrix details in [references/test_matrix.md](./references/test_matrix.md).
See mock implementation code in [examples/mocks.go](./examples/mocks.go).

---

## 4. Step-by-Step Testing Runbook

1. **Run Unit Tests**:
   ```bash
   go test -v ./engine/... ./config/...
   ```
2. **Run End-to-End Integration Suite with Mocks**:
   ```bash
   go test -v -race -timeout 30s ./test/...
   ```
3. **Verify Immediate Clearing Assertion**:
   - In `test/mock_livesplit.go`, maintain a FIFO queue of received messages.
   - When a `"split"` action is received, assert that the very next message is `""`.
4. **Simulate Connection Drop & Recovery**:
   - Start GotoSplitter with mock SNI server.
   - Stop mock SNI server, wait 1 second, restart mock SNI server.
   - Assert GotoSplitter re-attaches and resumes memory polling automatically.

---

## 5. Verification & Validation Checklist

- [ ] All unit and integration tests pass: `go test -race ./...`.
- [ ] Mock SNI server faithfully simulates `DeviceList`, `Attach`, and `GetAddress`.
- [ ] Mock LiveSplit client verifies the immediate action clearing requirement.
- [ ] No race conditions detected during multi-threaded polling tests.
- [ ] Memory footprint remains flat over extended test runs (no memory leaks).
