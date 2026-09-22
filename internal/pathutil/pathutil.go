// Package pathutil はログメッセージからの共通パス抽出を提供する。
package pathutil

import (
	"path/filepath"
	"strings"
)

// FromMessage は監視メッセージ(fswatch / log stream 等)から先頭パス候補を抽出する。
// 「/」で始まる最初のトークンを filepath.Clean して返す。見つからなければ空文字。
func FromMessage(message string) string {
	for _, tok := range strings.Fields(message) {
		if strings.HasPrefix(tok, "/") {
			return filepath.Clean(tok)
		}
	}
	return ""
}
