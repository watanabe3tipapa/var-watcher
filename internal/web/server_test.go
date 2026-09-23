package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/watanabe3tipapa/var-watcher/internal/config"
	"github.com/watanabe3tipapa/var-watcher/internal/diff"
	"github.com/watanabe3tipapa/var-watcher/internal/engine"
	"github.com/watanabe3tipapa/var-watcher/internal/perf"
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

func TestExportCSV(t *testing.T) {
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

	srv := NewServer(e)
	srv.SetStore(st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/export?format=csv&q=created")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Disposition"); !strings.Contains(ct, "attachment") {
		t.Fatalf("expected attachment disposition, got %q", ct)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.HasPrefix(string(body), "ts,source,level,message\n") {
		t.Fatalf("csv header missing: %q", body)
	}
	if !strings.Contains(string(body), "fswatch") {
		t.Fatalf("csv body missing row: %q", body)
	}
}

func TestExportJSON(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	now := time.Now()
	if err := st.Append(store.Line{TS: now, Source: "entr", Level: "stdout", Message: "/var/cache changed"}); err != nil {
		t.Fatalf("append: %v", err)
	}

	srv := NewServer(e)
	srv.SetStore(st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/export?format=json&source=entr")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var lines []store.Line
	if err := json.Unmarshal(body, &lines); err != nil {
		t.Fatalf("json decode: %v (%s)", err, body)
	}
	if len(lines) != 1 || lines[0].Message != "/var/cache changed" {
		t.Fatalf("unexpected json export: %+v", lines)
	}
}

func TestStatsAPI(t *testing.T) {
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

	srv := NewServer(e)
	srv.SetStore(st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/stats?hours=24")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var payload struct {
		Enabled bool        `json:"enabled"`
		Stats   store.Stats `json:"stats"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !payload.Enabled {
		t.Fatal("expected enabled=true with store set")
	}
	if payload.Stats.Total != 1 {
		t.Fatalf("expected 1 total, got %d", payload.Stats.Total)
	}
	if len(payload.Stats.Hourly) != 24 {
		t.Fatalf("expected 24 hourly buckets, got %d", len(payload.Stats.Hourly))
	}
}

func TestStatsAPIDisabled(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	srv := NewServer(e) // store 未設定
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/stats")
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

func TestTreeAPI(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	now := time.Now()
	for _, m := range []string{
		"/var/log/system.log Created IsFile",
		"/var/log/system.log Modified IsFile",
		"/var/cache/x Created",
	} {
		if err := st.Append(store.Line{TS: now, Source: "fswatch", Level: "stdout", Message: m}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	srv := NewServer(e)
	srv.SetStore(st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/tree?hours=24")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var payload struct {
		Enabled bool           `json:"enabled"`
		Tree    store.TreeNode `json:"tree"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !payload.Enabled {
		t.Fatal("expected enabled=true with store set")
	}
	if payload.Tree.Count != 3 {
		t.Fatalf("expected 3 total, got %d", payload.Tree.Count)
	}
	if payload.Tree.Name != "/" {
		t.Fatalf("expected root name /, got %q", payload.Tree.Name)
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

func TestDiffsAPI(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dm := diff.New(5)
	dm.Capture(path + " Modified") // 初回スナップショット
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := dm.Capture(path + " Modified"); d == nil {
		t.Fatal("expected a diff after modification")
	}

	srv := NewServer(e)
	srv.SetDiffs(dm)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/diffs")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), `"added":1`) {
		t.Fatalf("expected added count 1 in body: %s", body)
	}
}

func TestDiffsAPIDisabled(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	srv := NewServer(e)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/diffs")
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
		t.Fatal("expected enabled=false without manager")
	}
}

func TestPresetsAPI(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	srv := NewServer(e)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/presets")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var list struct {
		Presets []config.Preset `json:"presets"`
		Target  string          `json:"target"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list.Presets) == 0 {
		t.Fatal("expected presets list")
	}
	if list.Target != "/var" {
		t.Fatalf("target = %q, want /var", list.Target)
	}
}

func TestPerfAPI(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	pm := perf.New(10)
	pm.Count()
	go pm.Run(30 * time.Millisecond)
	time.Sleep(100 * time.Millisecond)

	srv := NewServer(e)
	srv.SetPerf(pm)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/perf")
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
	if !payload.Enabled {
		t.Fatal("expected enabled=true with perf monitor")
	}
	if !strings.Contains(string(body), `"samples"`) {
		t.Fatalf("expected samples field: %s", body)
	}
}

func TestPerfAPIDisabled(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	srv := NewServer(e)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/perf")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if strings.Contains(string(body), `"enabled":true`) {
		t.Fatalf("expected enabled=false without monitor, got: %s", body)
	}
}

func TestPresetApplyAPI(t *testing.T) {
	e := engine.New(config.Default(), engine.NewBus())
	srv := NewServer(e)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	res, err := http.Post(ts.URL+"/api/presets/caches/apply", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), `"applied":"caches"`) {
		t.Fatalf("expected applied caches: %s", body)
	}
	if e.Target() != "/var/folders" {
		t.Fatalf("engine target = %q, want /var/folders", e.Target())
	}

	res2, err := http.Post(ts.URL+"/api/presets/nope/apply", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown preset, got %d", res2.StatusCode)
	}
}
