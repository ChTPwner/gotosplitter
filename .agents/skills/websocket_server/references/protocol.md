# LiveSplit One WebSocket Protocol Reference

Authoritative sources: [`server_protocol.rs`](https://github.com/LiveSplit/livesplit-core/blob/f7dfa59aea312217652ebad62aae0ea4e789e3fc/src/networking/server_protocol.rs) at the LiveSplit One submodule's pinned commit, and [`LiveSplitServer.ts`](https://github.com/LiveSplit/LiveSplitOne/blob/master/src/api/LiveSplitServer.ts), which passes incoming text frames to that protocol handler.

## 1. Connection and command direction

- GotoSplitter hosts a WebSocket server; LiveSplit One connects as its client.
- The LiveSplit One client treats every incoming text frame as a `livesplit-core` JSON command.
- Commands and responses travel over the same connection. This is not a persistent action-state or metadata-query protocol.

## 2. Command frames

Send a JSON object in one text frame. The pinned `livesplit-core` `Command` enum uses `#[serde(tag = "command", rename_all = "camelCase")]`, so the wire discriminators are lower-camel-case:

```json
{"command":"start"}
{"command":"split"}
{"command":"splitOrStart"}
{"command":"pause"}
{"command":"resume"}
{"command":"undoSplit"}
{"command":"skipSplit"}
{"command":"getCurrentState"}
{"command":"getSegmentName","index":1}
{"command":"reset","saveAttempt":false}
```

The source comment's `{"command":"SplitOrStart"}` example conflicts with the enum's `camelCase` serde annotation. GotoSplitter follows the actual enum annotation, using lower-camel-case values, and tests assert those wire spellings. Bare text such as `"split"`, `"starttimer"`, or `""` is not a valid command.

`reset` takes an optional `saveAttempt` boolean. GotoSplitter asks the user to Save, Discard, or Cancel; Save sends `true`, Discard sends `false`, and Cancel sends nothing.

## 3. Responses and timer events

Successful commands return a JSON envelope:

```json
{"success":null}
{"success":"Segment Name"}
{"success":{"state":"Running","index":1}}
```

Errors use an envelope such as:

```json
{"error":{"code":"NoRunInProgress"}}
```

Timer events are separate JSON text frames, for example:

```json
{"event":"Splitted"}
```

An event can arrive before the success response for the command that caused it. Ignore event-only frames when waiting for a command result; do not treat them as a second command response. The protocol has no request ID, so serialize outstanding commands per WebSocket connection.

## 4. Current split tracking and metadata limits

1. Send `getCurrentState`.
2. For `Running` or `Paused`, read the returned `index` and send `getSegmentName` with that index.
3. Treat `NotRunning` and `Ended` as no active segment.

The protocol does not expose game or category metadata. Keep game-definition selection explicit in GotoSplitter; do not issue invented `getGameName`, `getCategoryName`, or `getCurrentSplitName` commands or expect raw-string replies.
