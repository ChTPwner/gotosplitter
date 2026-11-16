package livesplit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestActionCommandUsesSerdeCamelCaseDiscriminators(t *testing.T) {
	tests := []struct {
		action  string
		command string
	}{
		{action: "start", command: "start"},
		{action: "starttimer", command: "start"},
		{action: "split", command: "split"},
		{action: "splitorstart", command: "splitOrStart"},
		{action: "pause", command: "pause"},
		{action: "resume", command: "resume"},
		{action: "undo", command: "undoSplit"},
		{action: "skip", command: "skipSplit"},
		{action: "togglepauseorstart", command: "togglePauseOrStart"},
		{action: "undoallpauses", command: "undoAllPauses"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			command, err := actionCommand(tt.action)
			if err != nil {
				t.Fatalf("actionCommand(%q): %v", tt.action, err)
			}
			if command.Command != tt.command {
				t.Fatalf("command = %q, want %q", command.Command, tt.command)
			}
			message, err := encodeCommand(command)
			if err != nil {
				t.Fatalf("encode command: %v", err)
			}
			var wire struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(message, &wire); err != nil {
				t.Fatalf("decode encoded command %q: %v", message, err)
			}
			if wire.Command != tt.command {
				t.Fatalf("wire command = %q, want %q", wire.Command, tt.command)
			}
		})
	}
}

func TestDecodeProtocolResultSeparatesEventsAndResponses(t *testing.T) {
	event, err := decodeProtocolResult([]byte(`{"event":"Splitted"}`))
	if err != nil {
		t.Fatalf("decode timer event: %v", err)
	}
	if !event.Event || event.Err != nil {
		t.Fatalf("event result = %+v, want event without error", event)
	}

	success, err := decodeProtocolResult([]byte(`{"success":null}`))
	if err != nil {
		t.Fatalf("decode success: %v", err)
	}
	if success.Event || success.Err != nil || string(success.Success) != "null" {
		t.Fatalf("success result = %+v, want success null", success)
	}

	failure, err := decodeProtocolResult([]byte(`{"error":{"code":"NoRunInProgress"}}`))
	if err != nil {
		t.Fatalf("decode protocol error: %v", err)
	}
	if failure.Err == nil || !strings.Contains(failure.Err.Error(), "NoRunInProgress") {
		t.Fatalf("failure result = %+v, want NoRunInProgress", failure)
	}
}

func TestResetActionRequiresExplicitAttemptDecision(t *testing.T) {
	if _, err := actionCommand("reset"); err == nil {
		t.Fatal("expected reset action to require an explicit attempt-history decision")
	}
}
