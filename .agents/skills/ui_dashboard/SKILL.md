---
name: ui-dashboard
description: >-
  Builds and manages GotoSplitter's graphical user interface (Fyne desktop or embedded web dashboard)
  and configuration persistence. Use this skill when implementing status panels, definition/event
  file pickers, real-time split tracking views, config.json persistence, or headless/CLI fallback mode.
---

# UI & User Configuration Skill (`@ui-agent`)

This skill defines the technical standards, UI widget layout, state synchronization, and configuration persistence for the **User Interface & Settings** component of GotoSplitter, owned by `@ui-agent`.

---

## 1. Role & Architectural Boundaries

- **Owning Agent:** `@ui-agent`
- **Primary Packages:** `ui/`, `config/`
- **Key Files:** `ui/app.go`, `ui/status.go`, `ui/pickers.go`, `config/config.go`
- **Core Mission:** Provide a clean, responsive graphical UI (Fyne desktop or embedded local web dashboard) and manage persistent user preferences across restarts.

```mermaid
graph TD
    subgraph UI Component
        StatusPanel["Status Panel (SNI & LiveSplit Indicators)"]
        TrackingPanel["Active Split & Game Display"]
        Pickers["File Pickers (definitions/ & events/)"]
        Settings["Settings & Port Config"]
    end

    subgraph State & Persistence
        Store["config.json (Preferences)"]
        StateBus["Thread-Safe State Bus"]
    end

    StateBus --> StatusPanel
    StateBus --> TrackingPanel
    Pickers --> Store
    Settings --> Store
    Store --> Pickers
```

---

## 2. Core Responsibilities

1. **Connection Status Panel**:
   - Real-time indicator of **QUsb2Snes / SNI Status**:
     - `Disconnected` (Red)
     - `Connecting...` (Yellow)
     - `Attached to: <DeviceName>` (Green)
   - Real-time indicator of **LiveSplit One Client Status**:
     - `Waiting for LiveSplit One...` (Yellow)
     - `Connected (ws://localhost:16834)` (Green)

2. **Active Game & Split Monitor**:
   - Display the selected game-definition name and current split target name.
   - Do not assume LiveSplit One supplies game or category metadata; its official WebSocket protocol exposes timer state and segment names only.
   - Show trigger feedback / split counter.

3. **Definition & Events File Pickers**:
   - Definition Folder Selector: Choose the folder containing game-definition JSON files; default to `$HOME/.config/gotosplitter/`, the directory containing `config.json`.
   - Current Definition Selector: List non-recursive `.json` filenames in the selected folder, sorted alphabetically. If no filename is configured, select the first alphabetically.
   - Events File Selector: Auto-discover JSON files in `events/`, provide dropdown selection, and allow manual browse.
   - Notify `@coordinator` / `@event-engine-agent` whenever a new file is selected.

4. **Configuration Persistence (`config.json`)**:
   - Automatically save and reload user preferences:
     - `game_definition_folder`
     - `current_game_definition` (filename within the folder)
     - `event_file`
     - `sni_address` (default `localhost:8080`)
     - `livesplit_address` (default `localhost:16834`)
     - `target_device` (preferred SNES hardware/emulator)

5. **Non-Blocking UI Invariant**:
   > [!IMPORTANT]
   > The UI event loop (Fyne run loop or web server) MUST NEVER block on network I/O, SNI memory queries, or WebSocket handshakes.
   > **Implementation Rule:** All subsystem communication must be asynchronous via buffered channels or mutex-guarded state structs.

6. **Headless / CLI Fallback Mode**:
   - If a graphical display server (X11, Wayland, macOS Quartz) is unavailable (e.g. running on a headless server, Docker container, or CI), gracefully fallback to a clean CLI/terminal logger mode without crashing.

7. **Startup Definition Errors**:
   - If no viable game definition can be resolved and parsed from configuration, show the error in a visible startup window and print it to the terminal.
   - Exit with a failure status after the error is acknowledged or the UI closes.

8. **Reset Confirmation**:
   - When the user requests a timer reset, show a modal with Save attempt & reset, Discard attempt, and Cancel.
   - Cancel sends no reset command and leaves coordinator tracking unchanged.
   - Keep the UI responsive while the coordinator sends the command and waits for its protocol response.

---

## 3. UI Specifications & Layout

See widget layout specifications in [references/ui_spec.md](./references/ui_spec.md).
See reference UI binding code in [examples/ui_example.go](./examples/ui_example.go).

---

## 4. Step-by-Step Implementation Runbook

1. **Check Display Availability**:
   - If running headless or flag `-headless` is passed, start CLI logger instead of Fyne window.
2. **Initialize Configuration**:
   - Call `config.Load()` to retrieve the definition folder, selected filename, and network addresses.
3. **Build Window Layout**:
   - Top: Status badges (SNI status, LiveSplit status).
   - Middle: Current Game, Category, and Target Split label.
   - Bottom: File pickers (Dropdowns + Browse buttons) and Port inputs.
4. **Subscribe to State Updates**:
   - In a background goroutine, listen for status changes from the coordinator and update UI elements using `fyne.Do()` or channel ticks to ensure UI-thread safety.
5. **Persist Settings on Change**:
   - When the folder changes, refresh the filename list and default to its first alphabetical JSON file if the current filename is unavailable.
   - Persist the folder and filename together. Applying a new definition updates the coordinator and resets run tracking.

---

## 5. Verification & Validation Checklist

- [ ] Window renders smoothly without UI freezes when connecting or disconnecting from SNI.
- [ ] Status indicators update in real time when LiveSplit One connects or disconnects.
- [ ] Definition folder and filename persist to `config.json` and reload on app restart.
- [ ] An empty filename selects the first alphabetically sorted JSON file; an empty folder remains selectable in the UI.
- [ ] Missing or invalid startup definitions produce a visible UI error and a terminal message before the application exits.
- [ ] App starts in headless mode with `-headless` or when display is missing without panic.
