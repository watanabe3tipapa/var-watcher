package alert

import (
	"testing"
	"time"

	"github.com/watanabe3tipapa/var-watcher/internal/config"
	"github.com/watanabe3tipapa/var-watcher/internal/engine"
)

func testRules() []config.AlertRule {
	return []config.AlertRule{
		{ID: "burst", Name: "一時ファイル急増", Pattern: "/var/tmp", MinEvents: 3, WindowSec: 60},
		{ID: "syslog", Name: "system.log 変更", Pattern: "system.log", MinEvents: 1, WindowSec: 60},
	}
}

func TestManagerFiresOnThreshold(t *testing.T) {
	bus := engine.NewBus()
	m := NewManager(testRules(), bus)

	var fired []Fired
	fire := make(chan struct{}, 4)
	m.OnFire(func(f Fired, _ Rule) {
		fired = append(fired, f)
		fire <- struct{}{}
	})
	go m.Run()
	<-m.Ready()

	// 閾値 3 に満たない 2 件のみ → 発火しない
	bus.Publish(engine.LogLine{Source: "entr", Level: "stdout", Message: "/var/tmp/x created"})
	bus.Publish(engine.LogLine{Source: "entr", Level: "stdout", Message: "/var/tmp/y created"})
	select {
	case <-fire:
		t.Fatal("should not fire below threshold")
	case <-time.After(100 * time.Millisecond):
	}

	// 3 件目で発火
	bus.Publish(engine.LogLine{Source: "entr", Level: "stdout", Message: "/var/tmp/z created"})
	select {
	case <-fire:
	case <-time.After(time.Second):
		t.Fatal("expected alert to fire")
	}

	if len(fired) != 1 || fired[0].ID != "burst" || fired[0].Count != 3 {
		t.Fatalf("unexpected fired: %+v", fired)
	}
}

func TestManagerPatternAndHistory(t *testing.T) {
	bus := engine.NewBus()
	var cfgRules []config.AlertRule
	for _, r := range testRules() {
		cfgRules = append(cfgRules, r)
	}
	m := NewManager(cfgRules, bus)
	go m.Run()
	<-m.Ready()

	// system.log の発火
	bus.Publish(engine.LogLine{Source: "fswatch", Level: "stdout", Message: "system.log changed /var/log/system.log"})

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(m.History()) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	h := m.History()
	if len(h) != 1 || h[0].ID != "syslog" {
		t.Fatalf("expected 1 syslog fired, got %+v", h)
	}
}

func TestManagerSkipsAlertSource(t *testing.T) {
	bus := engine.NewBus()
	m := NewManager(testRules(), bus)
	var fired []Fired
	m.OnFire(func(f Fired, _ Rule) { fired = append(fired, f) })
	go m.Run()
	go func() {
		ch, unsub := bus.Subscribe()
		defer unsub()
		for range ch {
		}
	}()
	<-m.Ready()

	// 自分自身が発行する alert 行は再発火しない
	bus.Publish(engine.LogLine{Source: "alert", Level: "warn", Message: "alert: 一時ファイル急増 (3 events) — /var/tmp/z created"})
	time.Sleep(100 * time.Millisecond)
	if len(fired) != 0 {
		t.Fatalf("alert source should not trigger: %+v", fired)
	}
}
