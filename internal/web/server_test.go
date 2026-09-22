package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/watanabe3tipapa/var-watcher/internal/config"
	"github.com/watanabe3tipapa/var-watcher/internal/engine"
	"github.com/watanabe3tipapa/var-watcher/internal/store"
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

func TestLogsAPI(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	now := time.Now()
	if err := st.Append(store.Line{TS: now, Source: "fswatch", Level: "stdout", Message: "/var/log created"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := st.Append(store.Line{TS: now.Add(-time.Minute), Source: "entr", Level: "stdout", Message: "/var/cache changed"}); err != nil {
		t.Fatalf("append: %v", err)
	}

	srv := NewServer(e)
	srv.SetStore(st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/logs?q=created")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	var payload struct {
		Enabled bool         `json:"enabled"`
		Logs    []store.Line `json:"logs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !payload.Enabled {
		t.Fatal("expected enabled=true with store set")
	}
	if len(payload.Logs) != 1 || payload.Logs[0].Message != "/var/log created" {
		t.Fatalf("unexpected result: %+v", payload.Logs)
	}
}

func TestLogsAPIDisabled(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	srv := NewServer(e) // store 未設定
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/logs")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Enabled {
		t.Fatal("expected enabled=false without store")
	}
}
