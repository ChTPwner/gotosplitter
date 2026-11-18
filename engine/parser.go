// Package engine implements the evaluation engine for LiveSplit One conditions.
package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ConditionType represents the evaluation rule type
type ConditionType string

const (
	TypeEq   ConditionType = "eq"   // Byte equals
	TypeWeq  ConditionType = "weq"  // 16-bit word equals (little-endian)
	TypeWgt  ConditionType = "wgt"  // 16-bit word greater than (little-endian)
	TypeWlt  ConditionType = "wlt"  // 16-bit word less than (little-endian)
	TypeGt   ConditionType = "gt"   // Byte greater than
	TypeLt   ConditionType = "lt"   // Byte less than
	TypeBit  ConditionType = "bit"  // Bitwise non-zero: (memory & value) != 0
	TypeMore ConditionType = "more" // Conjunction: all sub-conditions must match concurrently
	TypeNext ConditionType = "next" // Sequence: sub-conditions must match sequentially
)

// Condition represents a single memory condition or composite condition
type Condition struct {
	Name    string        `json:"name,omitempty"`
	Address string        `json:"address,omitempty"`
	Value   string        `json:"value,omitempty"`
	Type    ConditionType `json:"type"`
	Active  string        `json:"active,omitempty"`
	More    []Condition   `json:"more,omitempty"`
	Next    []Condition   `json:"next,omitempty"`

	// Parsed numerical values for high-performance evaluation
	ParsedAddress uint32 `json:"-"`
	ParsedValue   uint64 `json:"-"`
}

// IGTConfig represents the in-game timer memory locations
type IGTConfig struct {
	Active         string `json:"active,omitempty"`
	Address        string `json:"address,omitempty"`
	Type           string `json:"type,omitempty"`
	FramesAddress  string `json:"framesAddress,omitempty"`
	SecondsAddress string `json:"secondsAddress,omitempty"`
	MinutesAddress string `json:"minutesAddress,omitempty"`
	HoursAddress   string `json:"hoursAddress,omitempty"`

	ParsedFramesAddress  uint32 `json:"-"`
	ParsedSecondsAddress uint32 `json:"-"`
	ParsedMinutesAddress uint32 `json:"-"`
	ParsedHoursAddress   uint32 `json:"-"`
}

// GameDefinition represents the root schema of a Usb2Snes-Livesplit-Definitions file
type GameDefinition struct {
	Name        string            `json:"name"`
	Autostart   *Condition        `json:"autostart,omitempty"`
	IGT         *IGTConfig        `json:"igt,omitempty"`
	Alias       map[string]string `json:"alias,omitempty"`
	Definitions []Condition       `json:"definitions"`
}

// ParseHexUint32 parses hex strings with or without "0x" / "0X" prefix
func ParseHexUint32(raw string) (uint32, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "0x")
	raw = strings.TrimPrefix(raw, "0X")
	if raw == "" {
		return 0, fmt.Errorf("empty hex string")
	}
	val, err := strconv.ParseUint(raw, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid hex uint32 %q: %w", raw, err)
	}
	return uint32(val), nil
}

// ParseHexUint64 parses hex strings with or without "0x" / "0X" prefix
func ParseHexUint64(raw string) (uint64, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "0x")
	raw = strings.TrimPrefix(raw, "0X")
	if raw == "" {
		return 0, fmt.Errorf("empty hex string")
	}
	val, err := strconv.ParseUint(raw, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hex uint64 %q: %w", raw, err)
	}
	return val, nil
}

// NormalizeWRAMAddress normalizes an address string or number into standard WRAM offset
func NormalizeWRAMAddress(addr uint32) uint32 {
	if addr >= 0xF50000 && addr <= 0xF6FFFF {
		return addr - 0xF50000
	}
	if addr >= 0x7E0000 && addr <= 0x7FFFFF {
		return addr - 0x7E0000
	}
	return addr
}

// ParseAndNormalizeCondition recursively parses hex fields in a condition and sub-conditions
func ParseAndNormalizeCondition(cond *Condition) error {
	if cond == nil {
		return nil
	}

	if cond.Address != "" {
		addr, err := ParseHexUint32(cond.Address)
		if err != nil {
			return fmt.Errorf("failed to parse condition address %q: %w", cond.Address, err)
		}
		cond.ParsedAddress = addr
	}

	if cond.Value != "" {
		val, err := ParseHexUint64(cond.Value)
		if err != nil {
			return fmt.Errorf("failed to parse condition value %q: %w", cond.Value, err)
		}
		cond.ParsedValue = val
	}

	// Validate Condition Type
	switch cond.Type {
	case TypeEq, TypeWeq, TypeWgt, TypeWlt, TypeGt, TypeLt, TypeBit, TypeMore, TypeNext:
	case "":
		// If type is not specified but subconditions exist, default to composite type
		if len(cond.More) > 0 {
			cond.Type = TypeMore
		} else if len(cond.Next) > 0 {
			cond.Type = TypeNext
		} else {
			cond.Type = TypeEq
		}
	default:
		return fmt.Errorf("unsupported condition type: %q", cond.Type)
	}

	// Recursively parse 'more' sub-conditions
	for i := range cond.More {
		if err := ParseAndNormalizeCondition(&cond.More[i]); err != nil {
			return fmt.Errorf("error in 'more' sub-condition %d: %w", i, err)
		}
	}

	// Recursively parse 'next' sub-conditions
	for i := range cond.Next {
		if err := ParseAndNormalizeCondition(&cond.Next[i]); err != nil {
			return fmt.Errorf("error in 'next' sub-condition %d: %w", i, err)
		}
	}

	return nil
}

// LoadDefinition loads and parses a game definition JSON file from path
func LoadDefinition(filePath string) (*GameDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read definition file: %w", err)
	}

	var def GameDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to unmarshal definition JSON: %w", err)
	}

	if def.Name == "" {
		return nil, fmt.Errorf("game definition is missing 'name' field")
	}

	// Parse autostart
	if def.Autostart != nil {
		if err := ParseAndNormalizeCondition(def.Autostart); err != nil {
			return nil, fmt.Errorf("invalid autostart definition: %w", err)
		}
	}

	// Parse IGT addresses if configured
	if def.IGT != nil {
		if def.IGT.FramesAddress != "" {
			def.IGT.ParsedFramesAddress, _ = ParseHexUint32(def.IGT.FramesAddress)
		}
		if def.IGT.SecondsAddress != "" {
			def.IGT.ParsedSecondsAddress, _ = ParseHexUint32(def.IGT.SecondsAddress)
		}
		if def.IGT.MinutesAddress != "" {
			def.IGT.ParsedMinutesAddress, _ = ParseHexUint32(def.IGT.MinutesAddress)
		}
		if def.IGT.HoursAddress != "" {
			def.IGT.ParsedHoursAddress, _ = ParseHexUint32(def.IGT.HoursAddress)
		}
	}

	// Parse all split definitions
	for i := range def.Definitions {
		if err := ParseAndNormalizeCondition(&def.Definitions[i]); err != nil {
			return nil, fmt.Errorf("invalid definition at index %d (%q): %w", i, def.Definitions[i].Name, err)
		}
	}

	return &def, nil
}
