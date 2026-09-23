package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/watanabe3tipapa/var-watcher/internal/config"
)

func TestBusFanout(t *testing.T) {
	b := NewBus()
	ch1, un1 := b.Subscribe()
	defer un1()
	ch2, _ := b.Subscribe()

	b.Publish(LogLine{Source: "s", Message: "hello"})
	select {
	case <-ch1:
	default:
		t.Fatal("subscriber1 missed message")
	}
	select {
	case <-ch2:
	default:
		t.Fatal("subscriber2 missed message")
	}
}

func TestWatcherStreamsAndStops(t *testing.T) {
	b := NewBus()
	ch, un := b.Subscribe()
	defer un()

	w := NewWatcher("fake", "sh", []string{"-c", "echo line1; echo line2; sleep 30"}, b)
	if err := w.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !w.Info().Enabled {
		t.Fatal("expected enabled after start")
	}

	var got []string
	done := make(chan struct{})
	go func() {
		for line := range ch {
			if line.Source == "fake" && strings.HasPrefix(line.Message, "line") {
				got = append(got, line.Message)
			}
			if len(got) >= 2 {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("did not receive 2 lines, got %v", got)
	}

	w.Stop()
	waitEnabledFalse(t, w)
}

func TestWatcherNotInstalled(t *testing.T) {
	b := NewBus()
	w := NewWatcher("nope", "this-binary-does-not-exist-xyz", nil, b)
	if err := w.Start(); err != ErrNotInstalled {
		t.Fatalf("expected ErrNotInstalled, got %v", err)
	}
	if w.Info().State != StateError.String() {
		t.Fatal("expected error state")
	}
}

func TestEngineListBuiltins(t *testing.T) {
	e := New(config.Default(), NewBus())
	infos := e.List()
	if len(infos) != 4 {
		t.Fatalf("expected 4 builtins, got %d", len(infos))
	}
	names := map[string]bool{}
	for _, i := range infos {
		names[i.Name] = true
		if !i.Builtin {
			t.Fatalf("%s should be builtin", i.Name)
		}
	}
	for _, n := range BuiltinNames() {
		if !names[n] {
			t.Fatalf("missing builtin %s", n)
		}
	}
}

func TestEngineApplyPreset(t *testing.T) {
	cfg := config.Default()
	e := New(cfg, NewBus())
	if len(e.Presets()) == 0 {
		t.Fatal("expected presets from config")
	}
	p, err := e.ApplyPreset("caches")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if p.ID != "caches" {
		t.Fatalf("preset id = %q", p.ID)
	}
	if e.Target() != "/var/folders" {
		t.Fatalf("target = %q, want /var/folders", e.Target())
	}
	if _, err := e.ApplyPreset("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown preset")
	}
}

func TestEngineApplyPresetRestartsWatchers(t *testing.T) {
	cfg := config.Default()
	bus := NewBus()
	e := New(cfg, bus)
	// logs プリセットは target /var/log、fswatch+logstream enabled。
	p, err := e.ApplyPreset("logs")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if p.ID != "logs" {
		t.Fatalf("preset id = %q", p.ID)
	}
	infos := e.List()
	enabled := map[string]bool{}
	for _, i := range infos {
		enabled[i.Name] = i.Enabled
	}
	if !enabled["fswatch"] {
		t.Fatal("fswatch should be running after logs preset")
	}
	if !enabled["logstream"] {
		t.Fatal("logstream should be running after logs preset")
	}
	for _, i := range infos {
		if i.Name == "fswatch" || i.Name == "logstream" {
			if i.State != "running" {
				t.Fatalf("%s state = %q, want running", i.Name, i.State)
			}
			_ = e.Stop(i.Name)
		}
	}
}

func TestRingCap(t *testing.T) {
	r := NewRing(3)
	for i := 0; i < 10; i++ {
		r.Add(strings.Repeat("x", i+1))
	}
	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
}

func waitEnabledFalse(t *testing.T, w *Watcher) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !w.Info().Enabled {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("watcher still enabled after Stop")
}

func TestBusDedupDropsDuplicate(t *testing.T) {
	b := NewBus()
	b.EnableDedup(500*time.Millisecond, 16)
	ch, un := b.Subscribe()
	defer un()

	b.Publish(LogLine{Source: "a", Level: "info", Message: "/var/x changed"})
	b.Publish(LogLine{Source: "b", Level: "info", Message: "/var/x changed"})

	select {
	case <-ch:
	default:
		t.Fatal("first event should be delivered")
	}
	select {
	case <-ch:
		t.Fatal("duplicate within window should be dropped")
	default:
	}
}

func TestBusDedupWindowExpiry(t *testing.T) {
	b := NewBus()
	b.EnableDedup(50*time.Millisecond, 16)
	ch, un := b.Subscribe()
	defer un()

	b.Publish(LogLine{Source: "a", Level: "info", Message: "/var/y changed"})
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("first event missing")
	}

	time.Sleep(80 * time.Millisecond)
	b.Publish(LogLine{Source: "b", Level: "info", Message: "/var/y changed"})
	select {
	case <-ch:
	default:
		t.Fatal("event after window expiry should be allowed")
	}
}

func TestBusDedupDisabledByZeroWindow(t *testing.T) {
	b := NewBus()
	b.EnableDedup(0, 16)
	ch, un := b.Subscribe()
	defer un()

	for i := 0; i < 2; i++ {
		b.Publish(LogLine{Source: "a", Level: "info", Message: "/var/z changed"})
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("event %d missing (dedup should be disabled)", i+1)
		}
	}
}

func TestDeduperLRUCap(t *testing.T) {
	d := newDeduper(time.Hour, 3)
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !d.Allow("k"+string(rune('0'+i)), now.Add(time.Duration(i)*time.Millisecond)) {
			t.Fatalf("new key k%d should be allowed", i)
		}
	}
	if d.Allow("k0", now.Add(4*time.Millisecond)) {
		t.Fatal("k0 within window should be duplicate")
	}

	// 4th key evicts least recently used (k1)
	if !d.Allow("k3", now.Add(5*time.Millisecond)) {
		t.Fatal("k3 should be allowed (new)")
	}
	if !d.Allow("k1", now.Add(6*time.Millisecond)) {
		t.Fatal("k1 was evicted, should be allowed again")
	}
}

func TestBusStampsTimestamp(t *testing.T) {
	b := NewBus()
	ch, un := b.Subscribe()
	defer un()

	b.Publish(LogLine{Source: "a", Level: "info", Message: "/var/ts changed"})
	select {
	case line := <-ch:
		if line.TS.IsZero() {
			t.Fatal("TS should be stamped by the bus")
		}
	case <-time.After(time.Second):
		t.Fatal("event missing")
	}
}
