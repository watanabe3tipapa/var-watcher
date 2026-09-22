// Package alert はルールベースの条件付きアラートを提供する。
// config の alerts 配列で定義したパターンに対して、時間窓内のイベント数が
// 閾値に達すると発火する。
package alert

import (
	"strings"
	"sync"
	"time"

	"github.com/watanabe3tipapa/var-watcher/internal/config"
	"github.com/watanabe3tipapa/var-watcher/internal/engine"
)

// Fired は発火したアラートの記録。
type Fired struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Count   int       `json:"count"`
	FiredAt time.Time `json:"fired_at"`
	Last    string    `json:"last,omitempty"`
}

// Rule は正規化済みのアラートルール。
type Rule struct {
	ID        string
	Name      string
	Pattern   string
	Source    string
	Level     string
	MinEvents int
	Window    time.Duration
	SoundName string
}

const historyCap = 100

// Manager は bus を購読し、ルールごとに時間窓内カウントでアラートを発火する。
type Manager struct {
	bus     *engine.Bus
	rules   []Rule
	onFire  func(Fired, Rule)
	mu      sync.Mutex
	track   map[string][]time.Time // rule id → 窓内のタイムスタンプ
	history []Fired
	ready   chan struct{}
}

// NewManager は config のルール群から Manager を組み立てる。
func NewManager(cfgRules []config.AlertRule, bus *engine.Bus) *Manager {
	rules := make([]Rule, 0, len(cfgRules))
	for _, r := range cfgRules {
		rules = append(rules, Rule{
			ID:        r.ID,
			Name:      r.Name,
			Pattern:   r.Pattern,
			Source:    r.Source,
			Level:     r.Level,
			MinEvents: r.MinEvents,
			Window:    time.Duration(r.WindowSec) * time.Second,
			SoundName: soundFor(r.Sound),
		})
	}
	return &Manager{
		bus:     bus,
		rules:   rules,
		track:   make(map[string][]time.Time),
		history: make([]Fired, 0, historyCap),
		ready:   make(chan struct{}),
	}
}

// Ready は Run が購読を開始したら閉じられるチャネルを返す(テスト用)。
func (m *Manager) Ready() <-chan struct{} {
	return m.ready
}

func soundFor(enable bool) string {
	if !enable {
		return ""
	}
	return "Basso"
}

// OnFire はアラート発火時のコールバック(履歴追加とは別に呼ばれる)。
func (m *Manager) OnFire(cb func(Fired, Rule)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onFire = cb
}

// Run は bus を購読して監視を開始する。ブロックするため goroutine で呼ぶ。
func (m *Manager) Run() {
	ch, unsub := m.bus.Subscribe()
	defer unsub()
	close(m.ready)
	for line := range ch {
		if line.Source == "alert" {
			continue
		}
		m.handle(line)
	}
}

func (m *Manager) handle(line engine.LogLine) {
	now := line.TS
	if now.IsZero() {
		now = time.Now()
	}

	var fired []Fired
	var rules []Rule
	m.mu.Lock()
	for i := range m.rules {
		r := m.rules[i]
		if !r.matches(line) {
			continue
		}
		times := m.track[r.ID]
		cutoff := now.Add(-r.Window)
		times = append(times, now)
		k := 0
		for _, t := range times {
			if !t.Before(cutoff) {
				times[k] = t
				k++
			}
		}
		times = times[:k]
		if len(times) >= r.MinEvents {
			f := Fired{ID: r.ID, Name: r.Name, Count: len(times), FiredAt: now, Last: line.Message}
			m.addHistory(f)
			fired = append(fired, f)
			rules = append(rules, r)
			times = times[:0] // 発火後はリセット(次の窓で再カウント)
		}
		m.track[r.ID] = times
	}
	cb := m.onFire
	m.mu.Unlock()

	if cb == nil {
		return
	}
	for i, f := range fired {
		cb(f, rules[i])
	}
}

// History は発火済みアラートの新→旧順コピーを返す。
func (m *Manager) History() []Fired {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Fired, 0, len(m.history))
	for i := len(m.history) - 1; i >= 0; i-- {
		out = append(out, m.history[i])
	}
	return out
}

func (m *Manager) addHistory(f Fired) {
	m.history = append(m.history, f)
	if len(m.history) > historyCap {
		m.history = m.history[len(m.history)-historyCap:]
	}
}

func (r Rule) matches(line engine.LogLine) bool {
	if r.Pattern != "" && !strings.Contains(line.Message, r.Pattern) {
		return false
	}
	if r.Source != "" && line.Source != r.Source {
		return false
	}
	if r.Level != "" && line.Level != r.Level {
		return false
	}
	return true
}
