// Package perf は監視エンジンの CPU / メモリ / イベント処理レートを周期サンプリングする。
package perf

import (
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	// DefaultMaxSamples は保持するサンプル数(既定 60 = 1 分)。
	DefaultMaxSamples = 60
	// MemWarnMB はメモリ警告の閾値(512 MiB 相当)。
	MemWarnMB = 512
)

// Sample は 1 秒間の計測値。
type Sample struct {
	TS         time.Time `json:"ts"`
	Events     int64     `json:"events"`     // この 1 秒間に処理したイベント数
	EventsSec  int64     `json:"events_sec"` // 毎秒換算(Events と同じ)
	AllocMB    uint64    `json:"alloc_mb"`   // ヒープ割り当て量(MiB)
	SysMB      uint64    `json:"sys_mb"`     // Go 全体の割り当て量(MiB)
	Goroutines int       `json:"goroutines"`
	CPUSec     float64   `json:"cpu_sec"` // CPU 使用(ユーザ+システム, 秒)
}

// Snapshot は直近サンプル一覧と警告をまとめた応答。
type Snapshot struct {
	Enabled   bool     `json:"enabled"`
	Warned    bool     `json:"warned"`
	MemWarnMB uint64   `json:"mem_warn_mb"`
	Samples   []Sample `json:"samples"`
}

// Monitor はサンプリングを保持する。main から新規作成し、イベントごとに Count を呼ぶ。
type Monitor struct {
	mu       sync.Mutex
	max      int
	events   atomic.Int64
	samples  []Sample
	warned   atomic.Bool
	lastCPU  float64
	warnOnce bool
}

// New は最大 max 件のサンプルを蓄積する Monitor を作る。
func New(max int) *Monitor {
	if max <= 0 {
		max = DefaultMaxSamples
	}
	return &Monitor{max: max}
}

// Count はイベント 1 件をカウントする。bus 購読側から毎行呼ぶ。
func (m *Monitor) Count() { m.events.Add(1) }

// Run は interval 毎にサンプルを採取して保持する。goroutine で開始する前提。
func (m *Monitor) Run(interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	for range time.Tick(interval) {
		m.sample()
	}
}

// sample は 1 サンプルを採取してリング内に追記する。
func (m *Monitor) sample() {
	events := m.events.Swap(0)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	var ru syscall.Rusage
	sec := 0.0
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err == nil {
		sec = secVal(&ru)
	}

	s := Sample{
		TS:         time.Now(),
		Events:     events,
		EventsSec:  events,
		AllocMB:    ms.Alloc / (1024 * 1024),
		SysMB:      ms.Sys / (1024 * 1024),
		Goroutines: runtime.NumGoroutine(),
		CPUSec:     sec,
	}

	if s.AllocMB >= MemWarnMB && !m.warnOnce {
		m.warnOnce = true
		m.warned.Store(true)
	}

	m.mu.Lock()
	m.samples = append(m.samples, s)
	if len(m.samples) > m.max {
		m.samples = m.samples[len(m.samples)-m.max:]
	}
	m.lastCPU = sec
	m.mu.Unlock()
}

// Snapshot は直近サンプルを新しい順で返す。警告状態も含む。
func (m *Monitor) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Sample, len(m.samples))
	copy(out, m.samples)
	return Snapshot{Enabled: true, Warned: m.Warned(), MemWarnMB: MemWarnMB, Samples: out}
}

// Warned はメモリ閾値超過の警告が発生済みか返す。
func (m *Monitor) Warned() bool { return m.warned.Load() }
