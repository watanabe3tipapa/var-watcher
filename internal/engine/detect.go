package engine

import (
	"os"
	"os/exec"
)

// Prerequisite は実行環境の前提チェック結果。
type Prerequisite struct {
	OK      bool
	Message string
}

// CheckPrerequisites は root 権限 / 依存コマンドの有無を調べる。
func CheckPrerequisites(watchers []*Watcher) []Prerequisite {
	var out []Prerequisite
	if os.Geteuid() != 0 {
		out = append(out, Prerequisite{
			OK:      false,
			Message: "root 権限なし: /var の大半を読むには sudo で実行してください",
		})
	} else {
		out = append(out, Prerequisite{OK: true, Message: "root 権限: OK"})
	}
	for _, w := range watchers {
		bin := w.Command
		checks := []string{bin}
		checks = append(checks, w.Check...)
		seen := map[string]bool{}
		for _, b := range checks {
			if seen[b] {
				continue
			}
			seen[b] = true
			if _, err := exec.LookPath(b); err != nil {
				out = append(out, Prerequisite{
					OK:      false,
					Message: b + " がありません: brew install " + b + " を実行してください",
				})
			}
		}
	}
	return out
}
