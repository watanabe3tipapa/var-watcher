package notify

import (
	"fmt"
	"os/exec"
	"strings"
)

// Show は macOS の osascript で通知を表示する。非対応環境ではエラーを返す。
func Show(msg string) error {
	if strings.TrimSpace(msg) == "" {
		return nil
	}
	script := fmt.Sprintf(
		`display notification %q with title %q`,
		msg, "var-watcher",
	)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}
