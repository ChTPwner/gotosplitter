package livesplit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestClientSendUsesLiveSplitJSONCommands(t *testing.T) {
	messages := make(chan string, 3)
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()

		for i := 0; i < 3; i++ {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				t.Errorf("read websocket message: %v", err)
				return
			}
			messages <- string(msg)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"success":null}`)); err != nil {
				t.Errorf("write protocol response: %v", err)
				return
			}
		}
	}))
	defer server.Close()

	client := NewClient("ws" + strings.TrimPrefix(server.URL, "http"))
	if err := client.Connect(); err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer client.Close()

	if err := client.Send("start"); err != nil {
		t.Fatalf("send start: %v", err)
	}
	if err := client.Send("split"); err != nil {
		t.Fatalf("send split: %v", err)
	}
	if err := client.ResetAttempt(true); err != nil {
		t.Fatalf("send reset: %v", err)
	}

	expected := []string{
		`{"command":"start"}`,
		`{"command":"split"}`,
		`{"command":"reset","saveAttempt":true}`,
	}
	for i := range expected {
		select {
		case msg := <-messages:
			if msg != expected[i] {
				t.Fatalf("expected %q at index %d, got %q", expected[i], i, msg)
			}
			var command map[string]any
			if err := json.Unmarshal([]byte(msg), &command); err != nil {
				t.Fatalf("message %q is not a JSON protocol command: %v", msg, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for websocket messages")
		}
	}
}
