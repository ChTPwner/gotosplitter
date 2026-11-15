# LiveSplit One WebSocket Protocol Reference

## 1. Overview & Connection Architecture

- **Role:** GotoSplitter = WebSocket Server, LiveSplit One = Client.
- **Default Endpoint:** `ws://localhost:16834` (or configurable in `config.json`).
- **Origin / CORS:** LiveSplit One is commonly hosted at `https://one.livesplit.org`. The WebSocket HTTP upgrader must allow this origin or accept all origins (`CheckOrigin: func(r *http.Request) bool { return true }`).

---

## 2. Timer Commands & Action Protocol

GotoSplitter sends plain text frames or JSON command envelopes containing standard LiveSplit actions:

| Action Command | Purpose | Effect in LiveSplit One |
| :--- | :--- | :--- |
| `"start"` | Start Timer | Starts the speedrun timer if currently stopped |
| `"split"` | Trigger Split | Splits the current segment and moves to next |
| `"reset"` | Reset Timer | Resets the timer to 00:00:00 |
| `"pause"` | Pause Timer | Pauses active timer |
| `"resume"` | Resume Timer | Resumes paused timer |
| `"undo"` | Undo Split | Moves back to the previous split |
| `"skip"` | Skip Split | Skips split without recording segment time |
| `""` | No-Op / Clear | Clears active trigger to prevent duplicate firing |

### The Immediate Action Clearing Requirement

```
Timeline:
[T0] Event Engine detects memory match for split condition
[T1] LiveSplit Server broadcasts text frame: "split"
[T2] LiveSplit Server immediately broadcasts text frame: ""
```

**Why this is mandatory:**
LiveSplit One's WebSocket integration checks the received action value. If GotoSplitter leaves the last sent message as `"split"`, reconnecting clients or re-evaluating clients will interpret `"split"` repeatedly. Sending `""` immediately ensures an edge-triggered split action.

---

## 3. Metadata Queries & Inbound Frames

When supported by the client version or when polling client state:
- **Client to Server:** LiveSplit One or custom scripts may send queries such as:
  - `"getCurrentSplitName"`
  - `"getGameName"`
  - `"getCategoryName"`
  - Or structured JSON payloads:
    ```json
    {
      "type": "query",
      "command": "getCurrentSplitName"
    }
    ```
- **Server Response:** GotoSplitter sends the requested string or JSON payload back on the client connection.
