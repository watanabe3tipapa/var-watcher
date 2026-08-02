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
