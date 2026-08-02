package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Target      string              `json:"target"`
	Enabled     map[string]bool     `json:"enabled"`
	Args        map[string][]string `json:"args"`
	Notify      bool                `json:"notify"`
	MaxLogLines int                 `json:"max_log_lines"`
}

func Default() *Config {
	return &Config{
		Target:      "/var",
		Enabled:     map[string]bool{},
		Args:        map[string][]string{},
		Notify:      false,
		MaxLogLines: 2000,
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
