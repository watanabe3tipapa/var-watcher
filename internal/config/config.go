package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AlertRule は条件付きアラートの 1 ルール。
type AlertRule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Pattern   string `json:"pattern"`
	Source    string `json:"source,omitempty"`
	Level     string `json:"level,omitempty"`
	MinEvents int    `json:"min_events"`
	WindowSec int    `json:"window_sec"`
	Sound     bool   `json:"sound,omitempty"`
}

type Config struct {
	Target        string              `json:"target"`
	Enabled       map[string]bool     `json:"enabled"`
	Args          map[string][]string `json:"args"`
	Notify        bool                `json:"notify"`
	MaxLogLines   int                 `json:"max_log_lines"`
	DedupMs       int                 `json:"dedup_ms"`
	DedupMax      int                 `json:"dedup_max"`
	DbPath        string              `json:"db_path"`
	RetentionDays int                 `json:"retention_days"`
	Alerts        []AlertRule         `json:"alerts"`
	Presets       []Preset            `json:"presets,omitempty"`
}

// Preset はあらかじめ定義した監視対象のセット。Target / Enabled / Filter を切り替える。
type Preset struct {
	ID      string          `json:"id"`
	Name    string          `json:"name,omitempty"`
	Target  string          `json:"target"`
	Enabled map[string]bool `json:"enabled,omitempty"`
	Filter  string          `json:"filter,omitempty"`
}

func Default() *Config {
	return &Config{
		Target:        "/var",
		Enabled:       map[string]bool{},
		Args:          map[string][]string{},
		Notify:        false,
		MaxLogLines:   2000,
		DedupMs:       500,
		DedupMax:      4096,
		RetentionDays: 30,
		Presets:       DefaultPresets(),
	}
}

// DefaultPresets は同梱する監視対象のプリセット例。
func DefaultPresets() []Preset {
	return []Preset{
		{
			ID:      "logs",
			Name:    "Log files",
			Target:  "/var/log",
			Enabled: map[string]bool{"fswatch": true, "logstream": true},
			Filter:  "",
		},
		{
			ID:      "caches",
			Name:    "Caches",
			Target:  "/var/folders",
			Enabled: map[string]bool{"fswatch": true, "watchman": true},
			Filter:  "",
		},
		{
			ID:      "temp",
			Name:    "Temp files",
			Target:  "/tmp",
			Enabled: map[string]bool{"fswatch": true},
			Filter:  "",
		},
	}
}

// ApplyPreset はプリセットの Target / Enabled / Filter を現在値へ反映する。なければエラー。
func (c *Config) ApplyPreset(id string) (*Preset, error) {
	for i := range c.Presets {
		if c.Presets[i].ID == id {
			p := &c.Presets[i]
			c.Target = p.Target
			for k := range c.Enabled {
				delete(c.Enabled, k)
			}
			for k, v := range p.Enabled {
				c.Enabled[k] = v
			}
			return p, nil
		}
	}
	return nil, fmt.Errorf("unknown preset: %s", id)
}

func DefaultPath() string {
	if p := os.Getenv("VARWATCH_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.varwatch/config.json"
	}
	return filepath.Join(home, ".varwatch", "config.json")
}

// ResolveDbPath はログ永続化先(SQLite)のパスを返す。未指定なら ~/.varwatch/varwatch.db。
func (c *Config) ResolveDbPath() string {
	if c.DbPath != "" {
		return c.DbPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.varwatch/varwatch.db"
	}
	return filepath.Join(home, ".varwatch", "varwatch.db")
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}
	cfg := Default()
	// json.Unmarshal は既存の map フィールドを再利用して書き込むため、
	// デフォルトプリセットとディスク値がマージされないよう先に nil へ戻す。
	cfg.Presets = nil
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.MaxLogLines <= 0 {
		cfg.MaxLogLines = 2000
	}
	if cfg.DedupMs < 0 {
		cfg.DedupMs = 0
	}
	if cfg.DedupMax <= 0 {
		cfg.DedupMax = 4096
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 30
	}
	if len(cfg.Presets) == 0 {
		cfg.Presets = DefaultPresets()
	}
	for i := range cfg.Alerts {
		if cfg.Alerts[i].MinEvents <= 0 {
			cfg.Alerts[i].MinEvents = 1
		}
		if cfg.Alerts[i].WindowSec <= 0 {
			cfg.Alerts[i].WindowSec = 60
		}
		if cfg.Alerts[i].ID == "" {
			cfg.Alerts[i].ID = cfg.Alerts[i].Name
		}
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
