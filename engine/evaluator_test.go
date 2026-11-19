package engine

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"
)

type countingMemoryReader struct {
	calls int
}

func (r *countingMemoryReader) ReadMemory(uint32, int) ([]byte, error) {
	r.calls++
	return []byte{0}, nil
}

func TestEvaluateRejectsEmptyAddressAndValueBeforeReading(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		value       string
		wantErrPart string
	}{
		{name: "address", value: "0x0", wantErrPart: "address"},
		{name: "blank address", address: "  ", value: "0x0", wantErrPart: "address"},
		{name: "value", address: "0x0", wantErrPart: "value"},
		{name: "blank value", address: "0x0", value: "  ", wantErrPart: "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &countingMemoryReader{}
			_, err := Evaluate(&Condition{
				Name:    "test",
				Address: tt.address,
				Value:   tt.value,
				Type:    TypeEq,
			}, reader)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("Evaluate() error = %v, want %s validation error", err, tt.wantErrPart)
			}
			if reader.calls != 0 {
				t.Fatalf("ReadMemory calls = %d, want 0 for invalid condition", reader.calls)
			}
		})
	}
}

func TestParseAndNormalizeConditionRejectsEmptyAddressAndValue(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		value       string
		wantErrPart string
	}{
		{name: "address", value: "0x0", wantErrPart: "address"},
		{name: "blank address", address: "  ", value: "0x0", wantErrPart: "address"},
		{name: "value", address: "0x0", wantErrPart: "value"},
		{name: "blank value", address: "0x0", value: "  ", wantErrPart: "value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ParseAndNormalizeCondition(&Condition{
				Address: tt.address,
				Value:   tt.value,
				Type:    TypeEq,
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("ParseAndNormalizeCondition() error = %v, want %s validation error", err, tt.wantErrPart)
			}
		})
	}
}

func TestEvaluateMonitoringLogRequiresDebugAndIsDeduplicatedByNormalizedAddress(t *testing.T) {
	var output bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previousOutput)

	t.Setenv(debugEnv, "")
	disabledCondition := &Condition{
		Address:       "0x7E0020",
		Value:         "0x00",
		Type:          TypeEq,
		ParsedAddress: 0x7E0020,
	}
	if _, err := Evaluate(disabledCondition, &countingMemoryReader{}); err != nil {
		t.Fatalf("Evaluate() with debug disabled error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("debug-disabled output = %q, want no output", output.String())
	}

	t.Setenv(debugEnv, "1")
	for _, address := range []uint32{0x7E0010, 0xF50010, 0x7E0011} {
		condition := &Condition{
			Address:       fmt.Sprintf("0x%X", address),
			Value:         "0x00",
			Type:          TypeEq,
			ParsedAddress: address,
		}
		if _, err := Evaluate(condition, &countingMemoryReader{}); err != nil {
			t.Fatalf("Evaluate() at 0x%X error = %v", address, err)
		}
	}

	if got := strings.Count(output.String(), "monitoring address"); got != 2 {
		t.Fatalf("monitoring log count = %d, want 2 (one per normalized address); output: %q", got, output.String())
	}
}
