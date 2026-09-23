package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPresets(t *testing.T) {
	d := Default()
	if d.Lang != "ja" {
		t.Fatalf("default lang = %q, want ja", d.Lang)
	}
	if d.Target != "/var" {
		t.Fatalf("default target = %q, want /var", d.Target)
	}
	if len(d.Presets) != 3 {
		t.Fatalf("presets = %d, want 3", len(d.Presets))
	}
}

func TestLoadLang(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{"", "ja"},
		{"en", "en"},
		{"ja", "ja"},
		{"fr", "ja"},
	}
	for _, c := range cases {
		path := filepath.Join(t.TempDir(), "c.json")
		data := `{"lang": "` + c.lang + `", "target": "/var"}`
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Lang != c.want {
			t.Errorf("lang %q → %q, want %q", c.lang, cfg.Lang, c.want)
		}
	}
}

func TestApplyPreset(t *testing.T) {
	cfg := Default()
	p, err := cfg.ApplyPreset("caches")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if cfg.Target != "/var/folders" {
		t.Fatalf("target = %q, want /var/folders", cfg.Target)
	}
	if !cfg.Enabled["fswatch"] || !cfg.Enabled["watchman"] {
		t.Fatalf("enabled = %v, want fswatch+watchman", cfg.Enabled)
	}
	if cfg.Enabled["logstream"] {
		t.Fatal("logstream should not be enabled by caches preset")
	}
	_ = p

	if _, err = cfg.ApplyPreset("nope"); err == nil {
		t.Fatal("expected error for unknown preset")
	}
}

func TestLoadKeepsPresets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"target": "/var", "enabled": {},"presets": [{"id":"x","name":"X","target":"/x","enabled":{"fswatch":true}}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := cfg.ApplyPreset("x")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Target != "/x" {
		t.Fatalf("target = %q, want /x", cfg.Target)
	}
	if p.Name != "X" {
		t.Fatalf("name = %q, want X", p.Name)
	}
}

func TestLoadEReplacePresets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.json")
	data := `{
  "target": "/var/log",
  "enabled": {"fswatch": true},
  "presets": [
    {"id":"logs","name":"Log files","target":"/tmp/log","enabled":{"fswatch":true}},
    {"id":"cache","name":"Caches","target":"/tmp/cache","enabled":{"fswatch":true,"logstream":true}}
  ]
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Presets) != 2 {
		t.Fatalf("presets = %d, want 2", len(cfg.Presets))
	}
	enabled := cfg.Presets[1].Enabled
	if len(enabled) != 2 {
		t.Fatalf("cache enabled = %v, want 2 entries", enabled)
	}
	if enabled["watchman"] || enabled["fswatch"] == false || enabled["logstream"] == false {
		t.Fatalf("cache enabled = %v, want fswatch+logstream only", enabled)
	}
}
