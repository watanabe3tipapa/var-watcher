package store

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/watanabe3tipapa/var-watcher/internal/pathutil"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestAppendAndQuery(t *testing.T) {
	st := openTestStore(t)

	base := time.Now().Add(-time.Hour)
	lines := []Line{
		{TS: base.Add(1 * time.Minute), Source: "fswatch", Level: "stdout", Message: "/var/log/a created"},
		{TS: base.Add(2 * time.Minute), Source: "entr", Level: "stdout", Message: "/var/log/b changed"},
		{TS: base.Add(3 * time.Minute), Source: "fswatch", Level: "stdout", Message: "/var/cache/c removed"},
	}
	for _, l := range lines {
		if err := st.Append(l); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// 全件: 新しい順
	got, err := st.Query(Query{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
	if got[0].Message != "/var/cache/c removed" {
		t.Fatalf("expected newest first, got %q", got[0].Message)
	}

	// キーワード検索
	got, err = st.Query(Query{Keyword: "created"})
	if err != nil {
		t.Fatalf("query keyword: %v", err)
	}
	if len(got) != 1 || got[0].Message != "/var/log/a created" {
		t.Fatalf("keyword search failed: %+v", got)
	}

	// ソースで絞り込み
	got, err = st.Query(Query{Source: "entr"})
	if err != nil {
		t.Fatalf("query source: %v", err)
	}
	if len(got) != 1 || got[0].Source != "entr" {
		t.Fatalf("source filter failed: %+v", got)
	}

	// 時系列範囲
	since := base.Add(90 * time.Second)
	got, err = st.Query(Query{Since: &since})
	if err != nil {
		t.Fatalf("query since: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows since, got %d", len(got))
	}
}

func TestPrune(t *testing.T) {
	st := openTestStore(t)

	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now().Add(-time.Hour)
	if err := st.Append(Line{TS: old, Source: "s", Message: "old line"}); err != nil {
		t.Fatalf("append old: %v", err)
	}
	if err := st.Append(Line{TS: recent, Source: "s", Message: "recent line"}); err != nil {
		t.Fatalf("append recent: %v", err)
	}

	n, err := st.Prune(24 * time.Hour)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted, got %d", n)
	}

	got, _ := st.Query(Query{})
	if len(got) != 1 || got[0].Message != "recent line" {
		t.Fatalf("prune kept wrong rows: %+v", got)
	}
}

func TestEscapeLike(t *testing.T) {
	st := openTestStore(t)
	if err := st.Append(Line{TS: time.Now(), Source: "s", Message: "100% full"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := st.Query(Query{Keyword: "100%"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("literal %% should match, got %d", len(got))
	}
}

func TestTree(t *testing.T) {
	st := openTestStore(t)

	now := time.Now()
	lines := []Line{
		{TS: now.Add(-time.Minute), Source: "fswatch", Level: "stdout", Message: "/var/log/system.log Created IsFile"},
		{TS: now.Add(-2 * time.Minute), Source: "fswatch", Level: "stdout", Message: "/var/log/system.log Modified IsFile"},
		{TS: now.Add(-3 * time.Minute), Source: "fswatch", Level: "stdout", Message: "/var/log/wifi.log Created IsFile"},
		{TS: now.Add(-4 * time.Minute), Source: "entr", Level: "stdout", Message: "/var/cache/com.apple/temp.dat Created"},
		{TS: now.Add(-5 * time.Minute), Source: "entr", Level: "stdout", Message: "no path here"},
		{TS: now.Add(-6 * time.Minute), Source: "fswatch", Level: "stdout", Message: "/tmp/alpha Created IsFile"},
		{TS: now.Add(-7 * time.Minute), Source: "fswatch", Level: "stdout", Message: "/tmp/beta Created IsFile"},
	}
	for _, l := range lines {
		if err := st.Append(l); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	root, err := st.Tree(24, 10000)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	if root.Name != "/" {
		t.Fatalf("root name: %q", root.Name)
	}
	if root.Count != 6 {
		t.Fatalf("root count should be 6 (pathless skipped), got %d", root.Count)
	}

	var find func(n *TreeNode, path string) *TreeNode
	find = func(n *TreeNode, path string) *TreeNode {
		if n.Path == path {
			return n
		}
		for _, c := range n.Children {
			if r := find(c, path); r != nil {
				return r
			}
		}
		return nil
	}

	logNode := find(root, "/var/log")
	if logNode == nil {
		t.Fatal("missing /var/log")
	}
	if logNode.Count != 3 {
		t.Fatalf("/var/log count: %d, want 3", logNode.Count)
	}
	logSystem := find(root, "/var/log/system.log")
	if logSystem == nil || logSystem.Count != 2 {
		t.Fatalf("system.log node wrong: %+v", logSystem)
	}
	cache := find(root, "/var/cache/com.apple")
	if cache == nil || cache.Count != 1 {
		t.Fatalf("cache node wrong: %+v", cache)
	}

	// 降順ソート確認: children[0] が最大 Count
	if len(root.Children) < 2 {
		t.Fatalf("expected >=2 top-level children, got %d", len(root.Children))
	}
	if root.Children[0].Count < root.Children[1].Count {
		t.Fatal("children not sorted by count desc")
	}
}

func TestParsePath(t *testing.T) {
	cases := map[string]string{
		"/private/tmp/a Created IsFile": "/private/tmp/a",
		"/var/log/system.log Modified":  "/var/log/system.log",
		"log stream Output: /var/log/x": "/var/log/x",
		"no path here":                  "",
		"relative/X Y":                  "",
	}
	for in, want := range cases {
		if got := pathutil.FromMessage(in); got != want {
			t.Errorf("pathutil.FromMessage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStats(t *testing.T) {
	st := openTestStore(t)

	now := time.Now()
	lines := []Line{
		{TS: now.Add(-time.Minute), Source: "fswatch", Level: "stdout", Message: "/var/log/a created"},
		{TS: now.Add(-2 * time.Minute), Source: "fswatch", Level: "stdout", Message: "/var/log/a created"},
		{TS: now.Add(-3 * time.Minute), Source: "entr", Level: "stdout", Message: "/var/cache/b changed"},
		{TS: now.Add(-3 * time.Hour), Source: "plugin:sample", Level: "stdout", Message: "/tmp/c removed"},
	}
	for _, l := range lines {
		if err := st.Append(l); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// 直近 2 時間の集計(= 先頭 3 件 / plugin は範囲外)
	s, err := st.Stats(2)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if s.Total != 3 {
		t.Fatalf("expected 3 total, got %d", s.Total)
	}
	if len(s.Hourly) != 24 {
		t.Fatalf("expected 24 hourly buckets, got %d", len(s.Hourly))
	}
	sum := int64(0)
	for _, b := range s.Hourly {
		sum += b.Count
	}
	if sum != 3 {
		t.Fatalf("hourly sum mismatch: %d", sum)
	}
	if len(s.Weekly) != 7 {
		t.Fatalf("expected 7 weekday buckets, got %d", len(s.Weekly))
	}

	if len(s.BySource) < 2 {
		t.Fatalf("expected >=2 sources, got %d", len(s.BySource))
	}
	if s.BySource[0].Source != "fswatch" {
		t.Fatalf("expected fswatch first, got %q", s.BySource[0].Source)
	}
	if len(s.TopPaths) == 0 {
		t.Fatal("expected top paths")
	}
	if s.TopPaths[0].Path != "/var/log/a" {
		t.Fatalf("expected /var/log/a top, got %q", s.TopPaths[0].Path)
	}
}

func TestExportAllJSON(t *testing.T) {
	st := openTestStore(t)
	base := time.Now().Add(-time.Hour)
	lines := []Line{
		{TS: base.Add(1 * time.Minute), Source: "fswatch", Level: "stdout", Message: "first"},
		{TS: base.Add(2 * time.Minute), Source: "entr", Level: "stdout", Message: "second"},
	}
	for _, l := range lines {
		if err := st.Append(l); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	all, err := st.ExportAll(Query{})
	if err != nil {
		t.Fatalf("export all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(all))
	}
	if all[0].Message != "first" {
		t.Fatalf("export should be oldest first, got %q", all[0].Message)
	}

	var buf bytes.Buffer
	if err := st.WriteJSON(&buf, all); err != nil {
		t.Fatalf("write json: %v", err)
	}
	var decoded []Line
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json decode: %v (%s)", err, buf.String())
	}
	if len(decoded) != 2 {
		t.Fatalf("json rows mismatch: %d", len(decoded))
	}
}

func TestExportCSV(t *testing.T) {
	st := openTestStore(t)
	if err := st.Append(Line{TS: time.Now(), Source: "fswatch", Level: "stdout", Message: `a, "quoted", line`}); err != nil {
		t.Fatalf("append: %v", err)
	}

	all, err := st.ExportAll(Query{})
	if err != nil {
		t.Fatalf("export all: %v", err)
	}
	var buf bytes.Buffer
	if err := st.WriteCSV(&buf, all); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "ts,source,level,message\n") {
		t.Fatalf("csv header missing: %q", out)
	}
	if !strings.Contains(out, `"a, ""quoted"", line"`) {
		t.Fatalf("csv quoting wrong: %q", out)
	}
}
