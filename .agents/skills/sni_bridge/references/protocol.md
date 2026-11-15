# QUsb2Snes & SNI Protocol Specification

## 1. Overview & Architecture

- **Role:** GotoSplitter = WebSocket Client, QUsb2Snes / SNI = WebSocket Server.
- **Default Endpoints:**
  - SNI (Super Nintendo Interface): `ws://localhost:8080` (or `ws://localhost:23074`)
  - QUsb2Snes: `ws://localhost:23074` (or `ws://localhost:8080`)
- **Default Address Space:** `"SNES"`

---

## 2. Opcode Definitions

All requests from the client to the server are formatted as JSON objects:
```json
{
  "Opcode": "<Command>",
  "Space": "SNES",
  "Operands": ["<Arg1>", "<Arg2>", "..."]
}
```

### 2.1 Name Registration (`Name`)
Registers the client name on the server for logging and device arbitration.
- **Request:**
  ```json
  {
    "Opcode": "Name",
    "Space": "SNES",
    "Operands": ["GotoSplitter"]
  }
  ```
- **Response:** (None or acknowledgment depending on version)

### 2.2 Device Discovery (`DeviceList`)
Lists all connected SNES physical hardware or emulators.
- **Request:**
  ```json
  {
    "Opcode": "DeviceList",
    "Space": "SNES"
  }
  ```
- **Response (JSON):**
  ```json
  {
    "Results": ["SD2SNES COM3", "RetroArch", "Snes9x"]
  }
  ```

### 2.3 Device Attachment (`Attach`)
Attaches client communication to a specific discovered device.
- **Request:**
  ```json
  {
    "Opcode": "Attach",
    "Space": "SNES",
    "Operands": ["RetroArch"]
  }
  ```

### 2.4 Read Memory (`GetAddress`)
Requests memory bytes from the attached device.
- **Request:**
  ```json
  {
    "Opcode": "GetAddress",
    "Space": "SNES",
    "Operands": ["F50010", "1"]
  }
  ```
  - `Operands[0]`: Hex address string (e.g. `"F50010"`, `"7E0010"`)
  - `Operands[1]`: Hex size string (e.g. `"1"` for 1 byte, `"10"` for 16 bytes)
- **Response:**
  - Standard QUsb2Snes / SNI replies with raw **Binary WebSocket frame** containing the requested bytes.

---

## 3. SNES Memory Mapping & Addresses

| Region | SNES Address Range | QUsb2Snes Offset | Purpose |
| :--- | :--- | :--- | :--- |
| WRAM (Low) | `$7E:0000 - $7E:1FFF` | `0xF50000 - 0xF51FFF` | Direct page, scratchpad, game state |
| WRAM (High)| `$7E:2000 - $7F:FFFF` | `0xF52000 - 0xF6FFFF` | Level data, sprites, extended state |
| SRAM / Save| `$70:0000 - $70:7FFF` | `0xE00000 - ...` | Battery-backed save RAM |
