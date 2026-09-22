// Package diff はファイル変更の前後比較(行単位 diff)と履歴管理を提供する。
package diff

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/watanabe3tipapa/var-watcher/internal/pathutil"
)

// Kind は変更行の種別。
type Kind string

const (
	// Added は追加された行(緑表示)。
	Added Kind = "added"
	// Removed は削除された行(赤表示)。
	Removed Kind = "removed"
	// Context は変化のない前後行(薄表示)。
	Context Kind = "context"
	// Header は差分の先頭行表示用。
	Header Kind = "header"
)

// Change は 1 行の差分。LineNo は表示用行番号。
type Change struct {
	Kind   Kind   `json:"kind"`
	Line   string `json:"line"`
	LineNo int    `json:"line_no"`
}

// Diff は 1 ファイルの前後比較結果。
type Diff struct {
	Path    string    `json:"path"`
	TS      time.Time `json:"ts"`
	Added   int       `json:"added"`
	Removed int       `json:"removed"`
	Changes []Change  `json:"changes"`
}

const (
	// maxFileBytes は diff 対象とするファイルの上限サイズ。
	maxFileBytes = 1 << 20 // 1 MiB
	// maxHistory は保持する差分履歴の最大件数。
	maxHistoryDefault = 50
	// maxHistoryMin は Manager に指定できる最小上限。
	maxHistoryMin = 1
	// maxHistoryMax は Manager に指定できる最大上限。
	maxHistoryMax = 500
)

// Manager はスナップショット比較と履歴を保持する。
type Manager struct {
	mu       sync.Mutex
	snap     map[string][]byte
	history  []*Diff
	maxH     int
	maxBytes int64
}

// New はスナップショット換算での履歴上限 maxHistory を持つ Manager を作る。
func New(maxHistory int) *Manager {
	if maxHistory < maxHistoryMin {
		maxHistory = maxHistoryDefault
	}
	if maxHistory > maxHistoryMax {
		maxHistory = maxHistoryMax
	}
	return &Manager{
		snap:     map[string][]byte{},
		history:  []*Diff{},
		maxH:     maxHistory,
		maxBytes: maxFileBytes,
	}
}

// Lookup は最新の max 件を新しい順で返す。max <= 0 なら全件。
func (m *Manager) Lookup(max int) []*Diff {
	m.mu.Lock()
	defer m.mu.Unlock()
	if max <= 0 || max > len(m.history) {
		max = len(m.history)
	}
	start := len(m.history) - max
	out := make([]*Diff, 0, max)
	for i := start; i < len(m.history); i++ {
		out = append(out, m.history[i])
	}
	return out
}

// Capture はメッセージからパスを抽出し、ファイル内容をスナップショットと比較する。
// 変更時に Diff を生成して返す。変更なし・対象外(バイナリ/巨大/不可読)は nil。
func (m *Manager) Capture(message string) *Diff {
	path := pathutil.FromMessage(message)
	if path == "" {
		return nil
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return nil
	}
	if fi.Size() > m.maxBytes {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil // バイナリは対象外(メタデータのみの扱い)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cur := normalize(data)
	prev, seen := m.snap[path]
	m.snap[path] = cur
	if !seen || bytes.Equal(prev, cur) {
		return nil
	}

	d := Compute(path, string(prev), string(cur))
	m.history = append(m.history, d)
	if len(m.history) > m.maxH {
		m.history = m.history[len(m.history)-m.maxH:]
	}
	return d
}

// Compute は prev と cur の行単位 diff を計算する。
func Compute(path, prev, cur string) *Diff {
	oldLines := splitLines(prev)
	newLines := splitLines(cur)

	// LCS を DP で求める(行単位)。上限で保護。
	if len(oldLines)*len(newLines) > 400000 {
		return &Diff{Path: path, TS: time.Now(), Changes: nil}
	}
	lcs := longestCommonSubsequence(oldLines, newLines)

	added, removed := 0, 0
	changes := []Change{}
	oi, ni := 0, 0
	for _, match := range lcs {
		for ; oi < match[0]; oi++ {
			changes = append(changes, Change{Kind: Removed, Line: oldLines[oi], LineNo: oi + 1})
			removed++
		}
		for ; ni < match[1]; ni++ {
			changes = append(changes, Change{Kind: Added, Line: newLines[ni], LineNo: ni + 1})
			added++
		}
		changes = append(changes, Change{Kind: Context, Line: newLines[ni], LineNo: ni + 1})
		oi, ni = match[0]+1, match[1]+1
	}
	for ; oi < len(oldLines); oi++ {
		changes = append(changes, Change{Kind: Removed, Line: oldLines[oi], LineNo: oi + 1})
		removed++
	}
	for ; ni < len(newLines); ni++ {
		changes = append(changes, Change{Kind: Added, Line: newLines[ni], LineNo: ni + 1})
		added++
	}

	return &Diff{
		Path:    path,
		TS:      time.Now(),
		Added:   added,
		Removed: removed,
		Changes: changes,
	}
}

// normalize は CRLF を LF に正規化する。
func normalize(b []byte) []byte {
	if bytes.IndexByte(b, '\r') < 0 {
		return b
	}
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
}

// splitLines は空行を保持したまま行分割する。
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// longestCommonSubsequence は old/new 間の最長共通部分列の添字ペアを返す。
func longestCommonSubsequence(oldLines, newLines []string) [][2]int {
	n, m := len(oldLines), len(newLines)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	out := [][2]int{}
	i, j := 0, 0
	for i < n && j < m {
		if oldLines[i] == newLines[j] {
			out = append(out, [2]int{i, j})
			i, j = i+1, j+1
		} else if dp[i+1][j] >= dp[i][j+1] {
			i++
		} else {
			j++
		}
	}
	return out
}
