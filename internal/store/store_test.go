package store

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
