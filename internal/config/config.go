package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type Config struct {
	Timezone        string                   `toml:"timezone"`
	RefreshInterval string                   `toml:"refresh_interval"`
	Currency        string                   `toml:"currency"`
	Paths           Paths                    `toml:"paths"`
	Pricing         map[string]usage.Pricing `toml:"pricing"`
}

type Paths struct {
	Claude []string `toml:"claude"`
	Codex  []string `toml:"codex"`
	Gemini []string `toml:"gemini"`
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Timezone:        "local",
		RefreshInterval: "3s",
		Currency:        "USD",
		Paths: Paths{
			Claude: []string{filepath.Join(home, ".claude", "projects")},
			Codex: []string{
				filepath.Join(home, ".codex", "sessions"),
				filepath.Join(home, ".codex", "archived_sessions"),
			},
			Gemini: []string{filepath.Join(home, ".gemini", "tmp")},
		},
		Pricing: map[string]usage.Pricing{},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = ConfigPath()
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, err
	}
	cfg.Paths.Claude = expandPaths(cfg.Paths.Claude)
	cfg.Paths.Codex = expandPaths(cfg.Paths.Codex)
	cfg.Paths.Gemini = expandPaths(cfg.Paths.Gemini)
	if cfg.RefreshInterval == "" {
		cfg.RefreshInterval = "3s"
	}
	if cfg.Currency == "" {
		cfg.Currency = "USD"
	}
	if cfg.Pricing == nil {
		cfg.Pricing = map[string]usage.Pricing{}
	}
	return cfg, nil
}

func (c Config) RefreshDuration() time.Duration {
	d, err := time.ParseDuration(c.RefreshInterval)
	if err != nil || d <= 0 {
		return 3 * time.Second
	}
	return d
}

func ConfigPath() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "agent-cockpit", "config.toml")
		}
	}
	return filepath.Join(home, ".config", "agent-cockpit", "config.toml")
}

func expandPaths(paths []string) []string {
	home, _ := os.UserHomeDir()
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if path == "~" {
			path = home
		} else if len(path) > 2 && path[:2] == "~/" {
			path = filepath.Join(home, path[2:])
		}
		out = append(out, filepath.Clean(path))
	}
	return out
}
