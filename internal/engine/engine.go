package engine

import (
	"fmt"
	"sort"
	"sync"

	"github.com/watanabe3tipapa/var-watcher/internal/config"
	"github.com/watanabe3tipapa/var-watcher/internal/notify"
)

type WatcherInfo struct {
	Name      string   `json:"name"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Enabled   bool     `json:"enabled"`
	State     string   `json:"state"`
	Error     string   `json:"error,omitempty"`
	Installed bool     `json:"installed"`
	Pid       int      `json:"pid"`
	Builtin   bool     `json:"builtin"`
}

type Engine struct {
	cfg      *config.Config
	bus      *Bus
	mu       sync.RWMutex
	watchers map[string]*Watcher
}

func New(cfg *config.Config, bus *Bus) *Engine {
	if bus == nil {
		bus = NewBus()
	}
	e := &Engine{cfg: cfg, bus: bus, watchers: make(map[string]*Watcher)}
	for _, w := range Builtins(cfg, bus) {
		e.watchers[w.Name] = w
	}
	e.applyConfig()
	return e
}

func (e *Engine) Bus() *Bus {
	return e.bus
}

// Watchers は現在の watcher 一覧を返す(前提チェック用)。
func (e *Engine) Watchers() []*Watcher {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Watcher, 0, len(e.watchers))
	for _, w := range e.watchers {
		out = append(out, w)
	}
	return out
}

// AddPlugin はプラグイン watcher を追加する。組み込み名と衝突時は追加しない。
func (e *Engine) AddPlugin(w *Watcher) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.watchers[w.Name]; ok {
		return false
	}
	e.watchers[w.Name] = w
	return true
}

func (e *Engine) Start(name string) error {
	e.mu.RLock()
	w, ok := e.watchers[name]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown watcher: %s", name)
	}
	err := w.Start()
	if err == nil {
		e.mu.Lock()
		e.cfg.Enabled[name] = true
		e.mu.Unlock()
	}
	return err
}

func (e *Engine) Stop(name string) error {
	e.mu.RLock()
	w, ok := e.watchers[name]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown watcher: %s", name)
	}
	w.Stop()
	e.mu.Lock()
	e.cfg.Enabled[name] = false
	e.mu.Unlock()
	return nil
}

func (e *Engine) List() []WatcherInfo {
	e.mu.RLock()
	names := make([]string, 0, len(e.watchers))
	for n := range e.watchers {
		names = append(names, n)
	}
	e.mu.RUnlock()
	sort.Strings(names)

	out := make([]WatcherInfo, 0, len(names))
	for _, n := range names {
		e.mu.RLock()
		w := e.watchers[n]
		builtin := isBuiltin(n)
		e.mu.RUnlock()
		info := w.Info()
		info.Builtin = builtin
		out = append(out, info)
	}
	return out
}

func (e *Engine) Logs() (<-chan LogLine, func()) {
	return e.bus.Subscribe()
}

func (e *Engine) Target() string {
	return e.cfg.Target
}

func (e *Engine) MaxLogLines() int {
	return e.cfg.MaxLogLines
}

func (e *Engine) Notify(msg string) error {
	if !e.cfg.Notify {
		return nil
	}
	return notify.Show(msg)
}

func (e *Engine) SaveConfig(path string) error {
	return e.cfg.Save(path)
}

// applyConfig は保存済み設定の enabled 状態を反映する。
func (e *Engine) applyConfig() {
	for _, name := range BuiltinNames() {
		if e.cfg.Enabled[name] {
			_ = e.Start(name)
		}
	}
}

func isBuiltin(name string) bool {
	for _, n := range BuiltinNames() {
		if n == name {
			return true
		}
	}
	return false
}
