# GotoSplitter

GotoSplitter connects an SNI/QUsb2Snes server to LiveSplit One and evaluates game definition files to trigger timer actions.

## Game definition settings

The settings file is stored at `$HOME/.config/gotosplitter/config.json`. Game definitions are configured with:

- `game_definition_folder`: the folder containing JSON definition files. By default, this is the same directory as `config.json`.
- `current_game_definition`: the selected JSON filename within that folder, not a full path.

For example:

```json
{
  "game_definition_folder": "/home/player/.config/gotosplitter",
  "current_game_definition": "super_metroid.json"
}
```

If `current_game_definition` is empty, GotoSplitter selects the first `.json` file alphabetically in the configured folder. The search is not recursive. Startup requires a viable definition: if the folder contains no JSON definitions, the selected file is missing, or the definition cannot be parsed, GotoSplitter displays the startup error in the UI, prints it in the terminal, and exits with a failure status. Correct the folder or filename in `config.json` and relaunch to recover.

Use the **Browse** or **Use folder** controls to change the folder, then choose a filename from **Current game definition**. Changes are saved to `config.json` and the selected definition is applied immediately; changing the definition resets the current run tracking. Existing configurations using the old `game_definition` file path are migrated to the folder and filename settings when loaded.

## Running

Run `go run .` to start GotoSplitter and open its settings window. The application starts its LiveSplit One WebSocket server at `ws://localhost:16834` and connects to SNI at `ws://127.0.0.1:23074` by default.
