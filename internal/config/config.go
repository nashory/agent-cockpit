package config

import (
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	ClaudePaths []string
	CodexPaths  []string
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		ClaudePaths: []string{filepath.Join(home, ".claude", "projects")},
		CodexPaths: []string{
			filepath.Join(home, ".codex", "sessions"),
			filepath.Join(home, ".codex", "archived_sessions"),
		},
	}
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
