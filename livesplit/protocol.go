package livesplit

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type protocolCommand struct {
	Command     string `json:"command"`
	Index       *int   `json:"index,omitempty"`
	SaveAttempt *bool  `json:"saveAttempt,omitempty"`
}

type protocolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type protocolResult struct {
	Success json.RawMessage
	Err     error
	Event   bool
}

func actionCommand(action string) (protocolCommand, error) {
	command := strings.ToLower(strings.TrimSpace(action))
	switch command {
	case "start", "starttimer":
		command = "start"
	case "split":
		command = "split"
	case "splitorstart":
		command = "splitOrStart"
	case "pause", "resume":
	case "reset":
		return protocolCommand{}, errors.New("reset requires an explicit save-attempt decision")
	case "undo", "undosplit":
		command = "undoSplit"
	case "skip", "skipsplit":
		command = "skipSplit"
	case "togglepauseorstart":
		command = "togglePauseOrStart"
	case "undoallpauses":
		command = "undoAllPauses"
	default:
		return protocolCommand{}, fmt.Errorf("unsupported LiveSplit One action %q", action)
	}

	return protocolCommand{Command: command}, nil
}

func encodeCommand(command protocolCommand) ([]byte, error) {
	message, err := json.Marshal(command)
	if err != nil {
		return nil, fmt.Errorf("encode LiveSplit One command: %w", err)
	}
	return message, nil
}

func decodeProtocolResult(message []byte) (protocolResult, error) {
	var envelope struct {
		Success json.RawMessage `json:"success"`
		Error   *protocolError  `json:"error"`
		Event   json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(message, &envelope); err != nil {
		return protocolResult{}, fmt.Errorf("decode LiveSplit One response: %w", err)
	}

	if envelope.Error != nil {
		message := envelope.Error.Message
		if message == "" {
			message = envelope.Error.Code
		}
		if message == "" {
			message = "unspecified protocol error"
		}
		return protocolResult{Err: fmt.Errorf("LiveSplit One command failed (%s): %s", envelope.Error.Code, message)}, nil
	}
	if envelope.Success != nil {
		return protocolResult{Success: envelope.Success}, nil
	}
	if envelope.Event != nil {
		return protocolResult{Event: true}, nil
	}
	return protocolResult{}, errors.New("LiveSplit One response has no success, error, or event field")
}
