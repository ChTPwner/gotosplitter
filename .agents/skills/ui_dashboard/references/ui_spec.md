# GotoSplitter UI Layout & Wireframe Specification

## 1. Window Wireframe (Fyne / Web)

```text
+-------------------------------------------------------------+
| GotoSplitter v1.0.0                                         |
+-------------------------------------------------------------+
| Connection Status:                                          |
|   SNI / QUsb2Snes : [ Attached: RetroArch ]       (GREEN)   |
|   LiveSplit One   : [ Connected (1 client) ]      (GREEN)   |
+-------------------------------------------------------------+
| Active Run Tracking:                                        |
|   Game      : Super Mario World                             |
|   Category  : 11 Exit                                       |
|   Current   : [ Yoshi's Island 1 ]                          |
+-------------------------------------------------------------+
| Files & Configuration:                                      |
|   Definition: [ super_mario_world.json         v ] [Browse] |
|   Events    : [ 11_exit_events.json            v ] [Browse] |
|   SNI Addr  : [ ws://localhost:8080                       ] |
|   LS Port   : [ 16834                                     ] |
+-------------------------------------------------------------+
| Log / Event Output:                                         |
|   [14:20:01] Attached to RetroArch                          |
|   [14:20:05] LiveSplit One connected                        |
|   [14:20:12] Triggered split: Yoshi's Island 1              |
+-------------------------------------------------------------+
```

---

## 2. Status Indicators & Colors

| Component | State | Color / Badge | Text Label |
| :--- | :--- | :--- | :--- |
| **SNI** | Disconnected | Red | `"Disconnected (Retrying...)"` |
| **SNI** | Connecting | Yellow / Amber | `"Connecting to ws://localhost:8080..."` |
| **SNI** | Attached | Green | `"Attached: <Device Name>"` |
| **LiveSplit** | Waiting | Yellow / Amber | `"Waiting for LiveSplit One client..."` |
| **LiveSplit** | Connected | Green | `"Connected (1 client)"` |

---

## 3. Configuration Store (`config.json`)

```json
{
  "sni_address": "localhost:8080",
  "livesplit_address": "ws://localhost:16834",
  "game_definition": "definitions/super_mario_world.json",
  "event_file": "events/super_mario_world_11_exit.json",
  "target_device": "RetroArch"
}
```
