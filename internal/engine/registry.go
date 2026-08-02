package engine

import (
	"encoding/json"

	"github.com/watanabe3tipapa/var-watcher/internal/config"
)

const (
	BuiltinFswatch   = "fswatch"
	BuiltinWatchman  = "watchman"
	BuiltinEntr      = "entr"
	BuiltinLogStream = "logstream"
)

func BuiltinNames() []string {
	return []string{BuiltinFswatch, BuiltinWatchman, BuiltinEntr, BuiltinLogStream}
}

// Builtins は config の Target / Args から組み込み 4 エンジンの Watcher を組み立てる。
func Builtins(cfg *config.Config, bus *Bus) []*Watcher {
	return []*Watcher{
		builtinFswatch(cfg, bus),
		builtinWatchman(cfg, bus),
		builtinEntr(cfg, bus),
		builtinLogStream(cfg, bus),
	}
}

func builtinFswatch(cfg *config.Config, bus *Bus) *Watcher {
	args := cfg.Args[BuiltinFswatch]
	if len(args) == 0 {
		args = []string{"-xr"}
	}
	return NewWatcher(BuiltinFswatch, "fswatch", append(args, cfg.Target), bus)
}

func builtinWatchman(cfg *config.Config, bus *Bus) *Watcher {
	req, _ := json.Marshal([]any{"subscribe", cfg.Target, "varwatch", map[string]any{"fields": []string{"name"}}})
	w := NewWatcher(BuiltinWatchman, "watchman", []string{"-j"}, bus)
	w.InitIn = append(req, '\n')
	return w
}

func builtinEntr(cfg *config.Config, bus *Bus) *Watcher {
	script := `find "$1" -type f 2>/dev/null | entr -d printf '%s changed\n'`
	w := NewWatcher(BuiltinEntr, "sh", []string{"-c", script, "sh", cfg.Target}, bus)
	w.Check = []string{"entr"}
	w.Installed = w.checkInstalled()
	return w
}

func builtinLogStream(cfg *config.Config, bus *Bus) *Watcher {
	predicate := `eventMessage CONTAINS "` + cfg.Target + `"`
	return NewWatcher(BuiltinLogStream, "log", []string{
		"stream", "--predicate", predicate, "--style", "syslog",
	}, bus)
}
