package livesplit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestClientSendNormalizesStartAndSplit(t *testing.T) {
	messages := make(chan string, 2)
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()

		for i := 0; i < 2; i++ {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				t.Errorf("read websocket message: %v", err)
				return
			}
			messages <- string(msg)
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

	expected := []string{"starttimer", "split"}
	for i := 0; i < 2; i++ {
		select {
		case msg := <-messages:
			if msg != expected[i] {
				t.Fatalf("expected %q at index %d, got %q", expected[i], i, msg)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for websocket messages")
		}
	}
}
