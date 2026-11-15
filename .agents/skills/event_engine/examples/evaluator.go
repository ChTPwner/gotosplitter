package examples

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

type ConditionType string

const (
	TypeEq   ConditionType = "eq"
	TypeWeq  ConditionType = "weq"
	TypeBit  ConditionType = "bit"
	TypeMore ConditionType = "more"
	TypeNext ConditionType = "next"
)

type Condition struct {
	Name    string        `json:"name"`
	Address string        `json:"address"`
	Value   string        `json:"value"`
	Type    ConditionType `json:"type"`
	More    []Condition   `json:"more,omitempty"`
	Next    []Condition   `json:"next,omitempty"`

	ParsedAddress uint32 `json:"-"`
	ParsedValue   uint64 `json:"-"`
}

type MemoryReader interface {
	ReadMemory(address uint32, size int) ([]byte, error)
}

func ParseAddress(raw string) (uint32, error) {
	raw = strings.TrimPrefix(raw, "0x")
	raw = strings.TrimPrefix(raw, "0X")
	val, err := strconv.ParseUint(raw, 16, 32)
	return uint32(val), err
}

func ParseValue(raw string) (uint64, error) {
	raw = strings.TrimPrefix(raw, "0x")
	raw = strings.TrimPrefix(raw, "0X")
	return strconv.ParseUint(raw, 16, 64)
}

// Evaluate checks if a single condition or composite condition is satisfied.
func Evaluate(cond *Condition, reader MemoryReader) (bool, error) {
	size := 1
	if cond.Type == TypeWeq {
		size = 2
	}

	data, err := reader.ReadMemory(cond.ParsedAddress, size)
	if err != nil {
		return false, fmt.Errorf("read failed at 0x%X: %w", cond.ParsedAddress, err)
	}
	if len(data) < size {
		return false, fmt.Errorf("insufficient data read: got %d, expected %d", len(data), size)
	}

	var matched bool
	switch cond.Type {
	case TypeEq:
		matched = uint64(data[0]) == cond.ParsedValue
	case TypeWeq:
		wordVal := uint64(binary.LittleEndian.Uint16(data))
		matched = wordVal == cond.ParsedValue
	case TypeBit:
		matched = (uint64(data[0]) & cond.ParsedValue) != 0
	default:
		return false, fmt.Errorf("unsupported condition type: %s", cond.Type)
	}

	if !matched {
		return false, nil
	}

	// Evaluate concurrent "more" conditions if present
	for i := range cond.More {
		childMatch, err := Evaluate(&cond.More[i], reader)
		if err != nil || !childMatch {
			return false, err
		}
	}

	return true, nil
}
