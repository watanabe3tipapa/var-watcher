package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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
	}
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
