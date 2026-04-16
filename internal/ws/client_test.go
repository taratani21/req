package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestConnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(websocket.TextMessage, msg)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := Connect(wsURL, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer conn.Close()

	err = conn.WriteMessage(websocket.TextMessage, []byte(`{"test": true}`))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(msg) != `{"test": true}` {
		t.Errorf("msg = %q, want %q", string(msg), `{"test": true}`)
	}
}

func TestConnect_WithHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), "Bearer test-token")
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		conn.Close()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	headers := map[string]string{"Authorization": "Bearer test-token"}
	conn, _, err := Connect(wsURL, headers, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	conn.Close()
}

func TestSendMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error: %v", err)
		}
		defer conn.Close()

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if string(msg) != `{"type": "subscribe"}` {
			t.Errorf("msg1 = %q", string(msg))
		}
		conn.WriteMessage(websocket.TextMessage, []byte(`{"status": "subscribed"}`))

		_, msg, err = conn.ReadMessage()
		if err != nil {
			return
		}
		if string(msg) != `{"type": "ping"}` {
			t.Errorf("msg2 = %q", string(msg))
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := Connect(wsURL, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer conn.Close()

	messages := []Message{
		{Payload: `{"type": "subscribe"}`, AwaitResponse: true},
		{Payload: `{"type": "ping"}`, AwaitResponse: false},
	}

	err = SendMessages(conn, messages, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
