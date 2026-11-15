# GotoSplitter Test Matrix & Verification Coverage

## 1. Test Matrix

| Area | Test Case | Expected Behavior |
| :--- | :--- | :--- |
| **LiveSplit WS** | Handshake & Connect | Client connects, server registers client |
| **LiveSplit WS** | Action Dispatch | `"split"` sent as text frame |
| **LiveSplit WS** | Immediate Action Clearing | Immediate `""` frame sent after action |
| **LiveSplit WS** | Disconnect Recovery | Server unregisters, doesn't panic on next split |
| **SNI Client** | DeviceList Discovery | Finds simulated devices `["RetroArch"]` |
| **SNI Client** | Attach to Target | Attach succeeds without error |
| **SNI Client** | GetAddress Byte Read | Returns exact byte at simulated address |
| **SNI Client** | Reconnect Backoff | Re-attempts connection after dropped socket |
| **Event Engine** | Condition `eq` | Matches exact byte comparison |
| **Event Engine** | Condition `weq` | Matches 16-bit little-endian word |
| **Event Engine** | Condition `bit` | Evaluates non-zero bitmask comparison |
| **Event Engine** | Condition `more` | Fires only when all sub-conditions match |
| **Event Engine** | Condition `next` | Advances chronological state machine |
| **Event Engine** | Single Address Poll | Only polls current active split address |
| **UI & Config** | Preference Persistence | `config.json` correctly reloads saved files |
| **System** | Headless Fallback | Runs cleanly in terminal when no display server |
| **System** | Concurrency Safety | Zero data races under `go test -race` |

---

## 2. Test Game Fixtures

- **Super Mario World**:
  - Autostart address `0x7E0100` = `0x14`
  - Overworld / Level Exit `0x7E1F2E` bit test
- **The Legend of Zelda: A Link to the Past**:
  - Intro / Escape split conditions
- **Super Metroid**:
  - Ceres Station escape sequence
