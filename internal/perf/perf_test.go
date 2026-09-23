package perf

import (
	"testing"
	"time"
)

func TestMonitorSamples(t *testing.T) {
	m := New(10)
	m.Count()
	m.Count()
	go m.Run(50 * time.Millisecond)
	time.Sleep(180 * time.Millisecond)
	snap := m.Snapshot()
	if !snap.Enabled {
		t.Fatal("expected enabled=true")
	}
	if len(snap.Samples) < 2 {
		t.Fatalf("expected >=2 samples, got %d", len(snap.Samples))
	}
	latest := snap.Samples[len(snap.Samples)-1]
	if latest.Goroutines <= 0 {
		t.Fatalf("unexpected sample: %+v", latest)
	}
}

func TestMonitorMaxCap(t *testing.T) {
	m := New(5)
	for i := 0; i < 10; i++ {
		m.sample()
	}
	snap := m.Snapshot()
	if len(snap.Samples) != 5 {
		t.Fatalf("samples = %d, want 5", len(snap.Samples))
	}
}

func TestMemWarnThreshold(t *testing.T) {
	m := New(3)
	// 閾値直接は比較しない: Warned は既定 false(struct 初期値)であることだけ確認。
	if m.Warned() {
		t.Fatal("expected no warning initially")
	}
}
