# Usb2Snes-Livesplit-Definitions & Events Schema Reference

## 1. Game Definition Schema (`definitions/*.json`)

Game definitions follow the standard `Usb2Snes-Livesplit-Definitions` specification:

```json
{
  "name": "Super Mario World",
  "autostart": {
    "name": "Start",
    "address": "0x7E0100",
    "value": "0x14",
    "type": "eq"
  },
  "definitions": [
    {
      "name": "Yoshi's Island 1",
      "address": "0x7E1F2E",
      "value": "0x01",
      "type": "bit"
    },
    {
      "name": "Bowser",
      "address": "0x7E0010",
      "value": "0x14",
      "type": "eq",
      "more": [
        {
          "address": "0x7E1DF5",
          "value": "0x10",
          "type": "eq"
        }
      ]
    }
  ],
  "alias": {
    "YI1": "Yoshi's Island 1"
  },
  "igt": {
    "address": "0x7E0F31",
    "type": "timer"
  }
}
```

---

## 2. Condition Evaluation Matrix

| Type | Name | Evaluation Rule |
| :--- | :--- | :--- |
| `eq` | Byte Equals | `memory[addr] == value` |
| `weq` | Word Equals | `(memory[addr] | (memory[addr+1] << 8)) == value` (16-bit little-endian) |
| `bit` | Bitwise AND | `(memory[addr] & value) != 0` |
| `more` | Conjunction | All child conditions in the `more` array must match concurrently |
| `next` | Sequence | Child conditions must occur chronologically; state machine advances step-by-step |

---

## 3. Ordered Fallback Events Schema (`events/*.json`)

Used when LiveSplit One split names cannot be read dynamically:

```json
{
  "game": "Super Mario World",
  "category": "11 Exit",
  "ordered_events": [
    {
      "event_name": "Start Run",
      "action": "start"
    },
    {
      "event_name": "Yoshi's Island 1",
      "action": "split"
    },
    {
      "event_name": "Yoshi's Island 2",
      "action": "split"
    },
    {
      "event_name": "Bowser",
      "action": "split"
    }
  ]
}
```
