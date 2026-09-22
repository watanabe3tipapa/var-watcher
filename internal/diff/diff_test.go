package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompute(t *testing.T) {
	d := Compute("/x", "a\nb\nc\n", "a\nB\nc\nd\n")
	if d.Added != 2 {
		t.Fatalf("added = %d, want 2", d.Added)
	}
	if d.Removed != 1 {
		t.Fatalf("removed = %d, want 1", d.Removed)
	}
	var added, removed, context int
	for _, c := range d.Changes {
		switch c.Kind {
		case Added:
			added++
		case Removed:
			removed++
		case Context:
			context++
		}
	}
	if added != 2 || removed != 1 || context != 2 {
		t.Fatalf("kinds counted incorrectly: added=%d removed=%d context=%d", added, removed, context)
	}
}

func TestComputeIdentical(t *testing.T) {
	d := Compute("/x", "a\nb\n", "a\nb\n")
	if d.Added != 0 || d.Removed != 0 {
		t.Fatalf("identical should have no changes: added=%d removed=%d", d.Added, d.Removed)
	}
}

func TestManagerCaptureAndHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(5)

	// 初回はスナップショット登録のみ(差分なし)
	if d := m.Capture(path + " Modified IsFile"); d != nil {
		t.Fatalf("first capture should be nil, got %+v", d)
	}
	if err := os.WriteFile(path, []byte("line1\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := m.Capture(path + " Modified IsFile")
	if d == nil {
		t.Fatal("expected diff after modification")
	}
	if d.Removed != 1 || d.Added != 1 {
		t.Fatalf("unexpected counts: added=%d removed=%d", d.Added, d.Removed)
	}
	if len(m.Lookup(0)) != 1 {
		t.Fatalf("history len = %d, want 1", len(m.Lookup(0)))
	}
}

func TestManagerSkipsBinaryAndHuge(t *testing.T) {
	dir := t.TempDir()

	bin := filepath.Join(dir, "bin.dat")
	if err := os.WriteFile(bin, []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(5)
	if d := m.Capture(bin + " Modified"); d != nil {
		t.Fatalf("binary should be skipped, got %+v", d)
	}

	huge := filepath.Join(dir, "huge.txt")
	if err := os.WriteFile(huge, make([]byte, maxFileBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := m.Capture(huge + " Modified"); d != nil {
		t.Fatalf("huge file should be skipped, got %+v", d)
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("a\n\nb\n")
	if len(got) != 3 || got[0] != "a" || got[1] != "" || got[2] != "b" {
		t.Fatalf("splitLines = %q", got)
	}
	if got := splitLines(""); got != nil {
		t.Fatalf("empty should be nil, got %q", got)
	}
}

func TestContextLinesGuard(t *testing.T) {
	// 巨大入力で Compute が破綻しないことを確認(時間重くない範囲)。
	n := 500
	old := strings.Repeat("x\n", n)
	cur := strings.Repeat("y\n", n)
	d := Compute("/big", old, cur)
	if d == nil {
		t.Fatal("expected a diff object")
	}
}
