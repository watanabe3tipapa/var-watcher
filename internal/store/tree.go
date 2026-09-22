package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/watanabe3tipapa/var-watcher/internal/pathutil"
)

// TreeNode はディレクトリツリーの 1 ノード。Count は自身と子孫の合計。
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Count    int64       `json:"count"`
	Children []*TreeNode `json:"children,omitempty"`
}

type treeNode struct {
	name  string
	count int64
	kids  map[string]*treeNode
}

// Tree は直近 hours 時間のイベントを message 先頭のパスで集計して階層化する。
// store はエンジン非依存のため、パス抽出は該当行のみを対象に単純に行う。
func (s *Store) Tree(hours int, limit int) (*TreeNode, error) {
	if hours <= 0 {
		hours = 24
	}
	if limit <= 0 {
		limit = 20000
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour).UnixMilli()

	rows, err := s.db.Query(
		`SELECT source, message FROM logs WHERE ts >= ? AND message != '' ORDER BY ts ASC LIMIT ?`,
		since, limit)
	if err != nil {
		return nil, fmt.Errorf("store: tree query: %w", err)
	}
	defer rows.Close()

	root := &treeNode{kids: map[string]*treeNode{}}
	for rows.Next() {
		var source, message string
		if err := rows.Scan(&source, &message); err != nil {
			return nil, fmt.Errorf("store: tree scan: %w", err)
		}
		p := pathutil.FromMessage(message)
		if p == "" {
			continue
		}
		emit(root, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := &TreeNode{Name: "/", Path: "/"}
	convertTree(root, "/", out)
	sortTree(out)
	return out, nil
}

// emit はパス p の各階層に件数を加算する。
func emit(root *treeNode, p string) {
	root.count++
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	cur := root
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		next, ok := cur.kids[part]
		if !ok {
			next = &treeNode{name: part, kids: map[string]*treeNode{}}
			cur.kids[part] = next
		}
		next.count++
		cur = next
	}
}

// convertTree は内部ツリーを出力用 TreeNode に変換する。
func convertTree(in *treeNode, parentPath string, out *TreeNode) {
	if in == nil {
		return
	}
	out.Count = in.count
	out.Children = make([]*TreeNode, 0, len(in.kids))
	for _, kid := range in.kids {
		child := &TreeNode{Name: kid.name, Path: filepath.Join(parentPath, kid.name)}
		convertTree(kid, child.Path, child)
		out.Children = append(out.Children, child)
	}
}

// sortTree は Count 降順、同数なら名前順に整列する。
func sortTree(n *TreeNode) {
	sort.Slice(n.Children, func(i, j int) bool {
		if n.Children[i].Count != n.Children[j].Count {
			return n.Children[i].Count > n.Children[j].Count
		}
		return n.Children[i].Name < n.Children[j].Name
	})
	for _, c := range n.Children {
		sortTree(c)
	}
}
