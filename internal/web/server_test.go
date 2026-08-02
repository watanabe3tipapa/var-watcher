package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/watanabe3tipapa/var-watcher/internal/config"
	"github.com/watanabe3tipapa/var-watcher/internal/engine"
)

func TestWebSocketStreamsLogs(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	srv := NewServer(e)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/logs"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// メッセージ受信の準備
	got := make(chan string, 8)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			got <- string(data)
		}
	}()

	// フェイク watcher を起動してログを流す
	w := engine.NewWatcher("fake", "sh", []string{"-c", "echo ws-hello"}, e.Bus())
	if !e.AddPlugin(w) {
		t.Fatal("add plugin")
	}
	if err := e.Start("fake"); err != nil {
		t.Fatalf("start: %v", err)
	}

	select {
	case msg := <-got:
		if !strings.Contains(msg, "ws-hello") {
			// "started" などの先行メッセージの場合は次の受信を待つ
			select {
			case msg2 := <-got:
				if !strings.Contains(msg2, "ws-hello") {
					t.Fatalf("unexpected messages: %s / %s", msg, msg2)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("no ws-hello message, got: %s", msg)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ws message received")
	}
}
