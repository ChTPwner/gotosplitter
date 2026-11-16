# GotoSplitter Architecture & Concurrency Model

## 1. System Block Diagram

```text
+------------------------------------------------------------------------+
|                               GotoSplitter                             |
|                                                                        |
|  +-------------------+                      +-----------------------+  |
|  |   livesplit.Server|<---[SplitAction]-----|     engine.Engine     |  |
|  | (ws://*:16834)    |                      | (Condition Evaluator) |  |
|  +--------^----------+                      +-----------^-----------+  |
|           |                                             |              |
|     Status Event                                  ReadMemory           |
|           |                                             |              |
|  +--------v----------+                      +-----------v-----------+  |
|  |      ui.App       |<---[TrackingState]---|      sni.Client       |  |
|  |  (Fyne/Web/CLI)   |<---[SNI Status]------| (ws://localhost:8080) |  |
|  +-------------------+                      +-----------------------+  |
+------------------------------------------------------------------------+
```

---

## 2. Interface Definitions

```go
// MemoryReader provides low-level memory read access to SNES RAM.
type MemoryReader interface {
    ReadMemory(address uint32, size int) ([]byte, error)
    IsConnected() bool
}

// ActionDispatcher broadcasts speedrun timer actions.
type ActionDispatcher interface {
    DispatchAction(action string) error
    ResetAttempt(saveAttempt bool) error
    IsClientConnected() bool
}

// SplitEngine processes memory conditions and emits split events.
type SplitEngine interface {
    Tick(reader MemoryReader) ([]string, error)
    SetTargetSplit(name string)
    GetTrackingInfo() TrackingInfo
}
```

---

## 3. Concurrency Rules & Invariants

1. **Non-Blocking Channel Sends**: Always guard channel writes with `select` or appropriate buffer sizing so that slow UI renders or network hitches do not block critical memory polling ticks.
2. **Deterministic Shutdown Order**:
   - Signal root context cancellation.
   - Stop memory polling ticker.
   - Flush pending actions and close LiveSplit client connections.
   - Disconnect SNI WebSocket.
   - Persist application configuration to disk.
3. **Data Races**: Always build and test with `go test -race ./...`. No shared state may be written without write lock or atomic primitives.

## 4. LiveSplit One protocol boundary

- The coordinator sends internal action names; `livesplit.Server` encodes them as JSON commands and reports protocol errors.
- Current split tracking uses `getCurrentState` followed by `getSegmentName`. Game/category metadata is not available from the official protocol.
- A reset requires a user choice: Save sends `saveAttempt: true`, Discard sends `false`, and Cancel sends no command and leaves tracking unchanged.
