// Package engine implements the evaluation engine for LiveSplit One conditions.
package engine

import (
	"encoding/binary"
	"fmt"
	"log"
)

// MemoryReader defines the standard interface for reading SNES memory
type MemoryReader interface {
	ReadMemory(address uint32, size int) ([]byte, error)
}

// Evaluate evaluates a condition and any concurrent 'more' conditions against a MemoryReader.
func Evaluate(cond *Condition, reader MemoryReader) (bool, error) {
	if cond == nil {
		return false, nil
	}
	if err := validateConditionFields(cond); err != nil {
		return false, err
	}

	size := 1
	if cond.Type == TypeWeq || cond.Type == TypeWgt || cond.Type == TypeWlt {
		size = 2
	}

	// Address normalization ensures address 0x0998, 0x7E0998, or 0xF50998 queries properly
	normalizedAddr := NormalizeWRAMAddress(cond.ParsedAddress)
	data, err := reader.ReadMemory(normalizedAddr, size)
	if err != nil {
		return false, fmt.Errorf("read failed at 0x%X: %w", cond.ParsedAddress, err)
	}
	if len(data) < size {
		return false, fmt.Errorf("insufficient data from read at 0x%X: got %d bytes, need %d", cond.ParsedAddress, len(data), size)
	}

	returnedValue := uint64(data[0])
	if size == 2 {
		returnedValue = uint64(binary.LittleEndian.Uint16(data))
	}
	log.Printf("monitoring address 0x%X (normalized to 0x%X, size=%d, type=%s): expected=0x%X returned=0x%X (bytes=% X)",
		cond.ParsedAddress, normalizedAddr, size, cond.Type, cond.ParsedValue, returnedValue, data[:size])

	var matched bool
	switch cond.Type {
	case TypeEq:
		matched = uint64(data[0]) == cond.ParsedValue
	case TypeGt:
		matched = uint64(data[0]) > cond.ParsedValue
	case TypeLt:
		matched = uint64(data[0]) < cond.ParsedValue
	case TypeWeq:
		wordVal := uint64(binary.LittleEndian.Uint16(data))
		matched = wordVal == cond.ParsedValue
	case TypeWgt:
		wordVal := uint64(binary.LittleEndian.Uint16(data))
		matched = wordVal > cond.ParsedValue
	case TypeWlt:
		wordVal := uint64(binary.LittleEndian.Uint16(data))
		matched = wordVal < cond.ParsedValue
	case TypeBit:
		matched = (uint64(data[0]) & cond.ParsedValue) != 0
	case TypeMore:
		// If base has an address and value, check them first
		if cond.Address != "" {
			matched = uint64(data[0]) == cond.ParsedValue
		} else {
			matched = true
		}
	default:
		return false, fmt.Errorf("unsupported condition type: %s", cond.Type)
	}

	if !matched {
		return false, nil
	}

	// Evaluate concurrent 'more' sub-conditions if present
	for i := range cond.More {
		childMatched, err := Evaluate(&cond.More[i], reader)
		if err != nil {
			return false, err
		}
		if !childMatched {
			return false, nil
		}
	}

	return true, nil
}

// NextTracker tracks progression of multi-step sequential conditions (such as Super Metroid autostart)
type NextTracker struct {
	currentStep int
}

// NewNextTracker initializes a new sequence tracker
func NewNextTracker() *NextTracker {
	return &NextTracker{currentStep: 0}
}

// Reset resets the tracker back to the initial step
func (t *NextTracker) Reset() {
	t.currentStep = 0
}

// Step returns the current sequence step index
func (t *NextTracker) Step() int {
	return t.currentStep
}

// Evaluate evaluates whether the sequence has completed or advanced
func (t *NextTracker) Evaluate(cond *Condition, reader MemoryReader) (bool, error) {
	if cond == nil {
		return false, nil
	}

	if len(cond.Next) == 0 {
		return Evaluate(cond, reader)
	}

	// Step 0: Base condition
	// Step 1..N: cond.Next[step - 1]
	totalSteps := 1 + len(cond.Next)

	var activeRule *Condition
	if t.currentStep == 0 {
		activeRule = cond
	} else if t.currentStep <= len(cond.Next) {
		activeRule = &cond.Next[t.currentStep-1]
	} else {
		return true, nil // Already completed
	}

	matched, err := Evaluate(activeRule, reader)
	if err != nil {
		return false, err
	}

	if matched {
		t.currentStep++
		if t.currentStep >= totalSteps {
			return true, nil
		}
	}

	return false, nil
}
