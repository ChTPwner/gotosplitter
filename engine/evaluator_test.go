package engine

import (
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
