package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/watanabe3tipapa/var-watcher/internal/engine"
)

// Discover は dir 配下の *.sh をスキャンし、エンジンに追加する watcher を返す。
func Discover(dir string, bus *engine.Bus) ([]*engine.Watcher, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*engine.Watcher
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		name := strings.TrimSuffix(e.Name(), ".sh")
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		out = append(out, engine.NewWatcher(
			fmt.Sprintf("plugin:%s", name),
			"sh",
			[]string{abs},
			bus,
		))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
