package livesplit

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestDispatchActionSendsOneJSONCommandWithoutClearingFrame(t *testing.T) {
	server, conn := startTestServer(t)

	result := make(chan error, 1)
	go func() {
		result <- server.DispatchAction("start")
	}()

	command := readCommand(t, conn)
	if command["command"] != "start" {
		t.Fatalf("command = %#v, want start", command)
	}
	writeEventAndSuccess(t, conn)
	if err := <-result; err != nil {
		t.Fatalf("dispatch start: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("received an unexpected clearing or follow-up frame")
	}
}

func TestResetAttemptIncludesExplicitSaveChoice(t *testing.T) {
	for _, saveAttempt := range []bool{true, false} {
		t.Run(map[bool]string{true: "save", false: "discard"}[saveAttempt], func(t *testing.T) {
			server, conn := startTestServer(t)

			result := make(chan error, 1)
			go func() {
				result <- server.ResetAttempt(saveAttempt)
			}()

			command := readCommand(t, conn)
			if command["command"] != "reset" || command["saveAttempt"] != saveAttempt {
				t.Fatalf("reset command = %#v, want reset with saveAttempt=%t", command, saveAttempt)
			}
			writeEventAndSuccess(t, conn)
			if err := <-result; err != nil {
				t.Fatalf("reset attempt: %v", err)
			}
		})
	}
}

func TestDispatchActionRequiresDecisionForReset(t *testing.T) {
	server := NewServer("127.0.0.1:0", nil)
	if err := server.DispatchAction("reset"); err == nil {
		t.Fatal("expected reset action to require an explicit attempt-history decision")
	}
}

func TestDispatchActionReturnsLiveSplitProtocolErrors(t *testing.T) {
	server, conn := startTestServer(t)

	result := make(chan error, 1)
	go func() {
		result <- server.DispatchAction("split")
	}()
	readCommand(t, conn)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"error":{"code":"NoRunInProgress"}}`)); err != nil {
		t.Fatalf("write protocol error: %v", err)
	}
	if err := <-result; err == nil || !strings.Contains(err.Error(), "NoRunInProgress") {
		t.Fatalf("dispatch error = %v, want NoRunInProgress", err)
	}
}

func TestRequestCurrentSplitNameUsesStateAndSegmentCommands(t *testing.T) {
	server, conn := startTestServer(t)
	statusUpdates := make(chan SplitStatus, 1)
	server.SetSplitCallback(func(status SplitStatus) {
		statusUpdates <- status
	})

	if err := server.RequestCurrentSplitName(); err != nil {
		t.Fatalf("request current split: %v", err)
	}
	stateCommand := readCommand(t, conn)
	if stateCommand["command"] != "getCurrentState" {
		t.Fatalf("state command = %#v, want getCurrentState", stateCommand)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"Started"}`)); err != nil {
		t.Fatalf("write timer event: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"success":{"state":"Running","index":1}}`)); err != nil {
		t.Fatalf("write current state: %v", err)
	}

	nameCommand := readCommand(t, conn)
	if nameCommand["command"] != "getSegmentName" || nameCommand["index"] != float64(1) {
		t.Fatalf("segment command = %#v, want getSegmentName index 1", nameCommand)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"success":"Landing Site"}`)); err != nil {
		t.Fatalf("write segment name: %v", err)
	}

	select {
	case status := <-statusUpdates:
		if !status.Running || status.CurrentSplitName != "Landing Site" {
			t.Fatalf("split status = %+v, want active Landing Site", status)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for current split status")
	}
}

func TestRequestCurrentSplitNameReportsNoActiveTimer(t *testing.T) {
	server, conn := startTestServer(t)
	statusUpdates := make(chan SplitStatus, 1)
	server.SetSplitCallback(func(status SplitStatus) {
		statusUpdates <- status
	})

	if err := server.RequestCurrentSplitName(); err != nil {
		t.Fatalf("request current split: %v", err)
	}
	readCommand(t, conn)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"success":{"state":"NotRunning"}}`)); err != nil {
		t.Fatalf("write current state: %v", err)
	}

	select {
	case status := <-statusUpdates:
		if status.Running || status.CurrentSplitName != "" {
			t.Fatalf("split status = %+v, want inactive timer", status)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for current split status")
	}

	if err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("requested a segment name while the timer was not running")
	}
}

func TestRequestCurrentSplitNameRequiresConnectedClient(t *testing.T) {
	server := NewServer("127.0.0.1:0", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop()

	if err := server.RequestCurrentSplitName(); err == nil {
		t.Fatal("expected an error when no LiveSplit client is connected")
	}
}

func startTestServer(t *testing.T) (*Server, *websocket.Conn) {
	t.Helper()
	server := NewServer("127.0.0.1:0", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Errorf("stop server: %v", err)
		}
	})

	u := url.URL{Scheme: "ws", Host: server.Addr()}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("connect LiveSplit client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	deadline := time.Now().Add(time.Second)
	for server.ConnectedClients() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if server.ConnectedClients() == 0 {
		t.Fatal("LiveSplit client did not connect")
	}
	return server, conn
}

func readCommand(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set command read deadline: %v", err)
	}
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read command: %v", err)
	}
	if messageType != websocket.TextMessage {
		t.Fatalf("command frame type = %d, want text", messageType)
	}
	var command map[string]any
	if err := json.Unmarshal(message, &command); err != nil {
		t.Fatalf("decode command %q: %v", message, err)
	}
	return command
}

func writeEventAndSuccess(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"Splitted"}`)); err != nil {
		t.Fatalf("write timer event: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"success":null}`)); err != nil {
		t.Fatalf("write command response: %v", err)
	}
}
