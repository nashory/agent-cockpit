package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type Config struct {
	Timezone        string                   `toml:"timezone"`
	RefreshInterval string                   `toml:"refresh_interval"`
	Currency        string                   `toml:"currency"`
	Budget          usage.Budget             `toml:"budget"`
	Limits          usage.Limits             `toml:"limits"`
	Paths           Paths                    `toml:"paths"`
	Pricing         map[string]usage.Pricing `toml:"pricing"`
}

type Paths struct {
	Claude   []string `toml:"claude"`
	Codex    []string `toml:"codex"`
	Gemini   []string `toml:"gemini"`
	OpenCode []string `toml:"opencode"`
	Amp      []string `toml:"amp"`
	Copilot  []string `toml:"copilot"`
	Kimi     []string `toml:"kimi"`
	Qwen     []string `toml:"qwen"`
	Codebuff []string `toml:"codebuff"`
	Kilo     []string `toml:"kilo"`
	Goose    []string `toml:"goose"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
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
			OpenCode: []string{
				filepath.Join(home, ".local", "share", "opencode"),
				filepath.Join(home, ".opencode"),
			},
			Amp: []string{filepath.Join(home, ".local", "share", "amp")},
			Copilot: []string{
				filepath.Join(home, ".copilot", "otel"),
			},
			Kimi: []string{filepath.Join(home, ".kimi")},
			Qwen: []string{filepath.Join(home, ".qwen")},
			Codebuff: []string{
				filepath.Join(home, ".config", "manicode"),
				filepath.Join(home, ".config", "manicode-dev"),
				filepath.Join(home, ".config", "manicode-staging"),
			},
			Kilo: []string{filepath.Join(home, ".local", "share", "kilo")},
			Goose: []string{
				filepath.Join(home, ".local", "share", "goose", "sessions"),
				filepath.Join(home, "Library", "Application Support", "goose", "sessions"),
				filepath.Join(home, ".local", "share", "Block", "goose", "sessions"),
			},
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
	cfg.Paths.OpenCode = expandPaths(cfg.Paths.OpenCode)
	cfg.Paths.Amp = expandPaths(cfg.Paths.Amp)
	cfg.Paths.Copilot = expandPaths(cfg.Paths.Copilot)
	cfg.Paths.Kimi = expandPaths(cfg.Paths.Kimi)
	cfg.Paths.Qwen = expandPaths(cfg.Paths.Qwen)
	cfg.Paths.Codebuff = expandPaths(cfg.Paths.Codebuff)
	cfg.Paths.Kilo = expandPaths(cfg.Paths.Kilo)
	cfg.Paths.Goose = expandPaths(cfg.Paths.Goose)
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

func SchemaJSON() ([]byte, error) {
	return json.MarshalIndent(configSchema(), "", "  ")
}

func ValidateFile(path string) []ValidationError {
	if path == "" {
		path = ConfigPath()
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return []ValidationError{{Field: "config", Message: fmt.Sprintf("config file not found: %s", path)}}
	}
	cfg := Default()
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return []ValidationError{{Field: "config", Message: err.Error()}}
	}
	var errs []ValidationError
	for _, key := range md.Undecoded() {
		errs = append(errs, ValidationError{Field: strings.Join(key, "."), Message: "unknown config key"})
	}
	errs = append(errs, Validate(cfg)...)
	return errs
}

func Validate(cfg Config) []ValidationError {
	var errs []ValidationError
	if cfg.Timezone != "" && cfg.Timezone != "local" {
		if _, err := time.LoadLocation(cfg.Timezone); err != nil {
			errs = append(errs, ValidationError{Field: "timezone", Message: "must be \"local\" or a valid IANA timezone"})
		}
	}
	if cfg.RefreshInterval != "" {
		d, err := time.ParseDuration(cfg.RefreshInterval)
		if err != nil || d <= 0 {
			errs = append(errs, ValidationError{Field: "refresh_interval", Message: "must be a positive Go duration, for example 3s"})
		}
	}
	if cfg.Currency == "" {
		errs = append(errs, ValidationError{Field: "currency", Message: "must not be empty"})
	}
	errs = append(errs, validateThresholds("budget", cfg.Budget.WarnPct, cfg.Budget.CriticalPct)...)
	errs = append(errs, validateThresholds("limits", cfg.Limits.WarnPct, cfg.Limits.CriticalPct)...)
	if cfg.Budget.DailyUSD < 0 {
		errs = append(errs, ValidationError{Field: "budget.daily_usd", Message: "must be non-negative"})
	}
	if cfg.Budget.WeeklyUSD < 0 {
		errs = append(errs, ValidationError{Field: "budget.weekly_usd", Message: "must be non-negative"})
	}
	if cfg.Budget.MonthlyUSD < 0 {
		errs = append(errs, ValidationError{Field: "budget.monthly_usd", Message: "must be non-negative"})
	}
	if cfg.Limits.Claude5HTokens < 0 {
		errs = append(errs, ValidationError{Field: "limits.claude_5h_tokens", Message: "must be non-negative"})
	}
	if cfg.Limits.Claude7DTokens < 0 {
		errs = append(errs, ValidationError{Field: "limits.claude_7d_tokens", Message: "must be non-negative"})
	}
	for source, paths := range map[string][]string{"paths.claude": cfg.Paths.Claude, "paths.codex": cfg.Paths.Codex, "paths.gemini": cfg.Paths.Gemini, "paths.opencode": cfg.Paths.OpenCode, "paths.amp": cfg.Paths.Amp, "paths.copilot": cfg.Paths.Copilot, "paths.kimi": cfg.Paths.Kimi, "paths.qwen": cfg.Paths.Qwen, "paths.codebuff": cfg.Paths.Codebuff, "paths.kilo": cfg.Paths.Kilo, "paths.goose": cfg.Paths.Goose} {
		for i, path := range paths {
			if strings.TrimSpace(path) == "" {
				errs = append(errs, ValidationError{Field: fmt.Sprintf("%s[%d]", source, i), Message: "path must not be empty"})
			}
		}
	}
	for model, price := range cfg.Pricing {
		prefix := fmt.Sprintf("pricing.%q", model)
		if strings.TrimSpace(model) == "" {
			errs = append(errs, ValidationError{Field: "pricing", Message: "model key must not be empty"})
		}
		if price.InputPerMillion < 0 {
			errs = append(errs, ValidationError{Field: prefix + ".input_per_million", Message: "must be non-negative"})
		}
		if price.OutputPerMillion < 0 {
			errs = append(errs, ValidationError{Field: prefix + ".output_per_million", Message: "must be non-negative"})
		}
		if price.CacheReadPerMillion < 0 {
			errs = append(errs, ValidationError{Field: prefix + ".cache_read_per_million", Message: "must be non-negative"})
		}
		if price.CacheWritePerMillion < 0 {
			errs = append(errs, ValidationError{Field: prefix + ".cache_write_per_million", Message: "must be non-negative"})
		}
	}
	return errs
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

func validateThresholds(section string, warn, critical float64) []ValidationError {
	var errs []ValidationError
	if warn < 0 || warn > 100 {
		errs = append(errs, ValidationError{Field: section + ".warn_pct", Message: "must be between 0 and 100"})
	}
	if critical < 0 || critical > 100 {
		errs = append(errs, ValidationError{Field: section + ".critical_pct", Message: "must be between 0 and 100"})
	}
	if warn > 0 && critical > 0 && warn > critical {
		errs = append(errs, ValidationError{Field: section + ".warn_pct", Message: "must be less than or equal to critical_pct"})
	}
	return errs
}

func configSchema() map[string]any {
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://github.com/nashory/agent-cockpit/schema/config.schema.json",
		"title":                "agent-cockpit config",
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"timezone":         map[string]any{"type": "string", "description": "IANA timezone name, or local."},
			"refresh_interval": map[string]any{"type": "string", "description": "Go duration used by live mode, for example 3s."},
			"currency":         map[string]any{"type": "string", "description": "Display currency label for estimated costs."},
			"budget":           thresholdBudgetSchema(),
			"limits":           thresholdLimitsSchema(),
			"paths": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"claude":   stringArraySchema("Claude Code project log roots."),
					"codex":    stringArraySchema("Codex session log roots."),
					"gemini":   stringArraySchema("Gemini temporary session roots."),
					"opencode": stringArraySchema("OpenCode data roots containing opencode.db, opencode-*.db, or storage/message JSON files."),
					"amp":      stringArraySchema("Amp data roots containing threads JSON files."),
					"copilot":  stringArraySchema("GitHub Copilot CLI OpenTelemetry JSONL export directories."),
					"kimi":     stringArraySchema("Kimi data roots containing sessions/*/*/wire.jsonl files."),
					"qwen":     stringArraySchema("Qwen Code data roots containing projects/*/chats/*.jsonl files."),
					"codebuff": stringArraySchema("Codebuff data roots containing projects/*/chats/*/chat-messages.json files."),
					"kilo":     stringArraySchema("Kilo Code data roots containing kilo.db."),
					"goose":    stringArraySchema("Goose data roots containing sessions.db."),
				},
			},
			"pricing": map[string]any{
				"type":                 "object",
				"description":          "Model substring pricing overrides, in USD per million tokens.",
				"additionalProperties": pricingSchema(),
			},
		},
	}
}

func thresholdBudgetSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"daily_usd":    nonNegativeNumberSchema(),
			"weekly_usd":   nonNegativeNumberSchema(),
			"monthly_usd":  nonNegativeNumberSchema(),
			"warn_pct":     percentSchema(),
			"critical_pct": percentSchema(),
		},
	}
}

func thresholdLimitsSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"claude_5h_tokens": map[string]any{"type": "integer", "minimum": 0},
			"claude_7d_tokens": map[string]any{"type": "integer", "minimum": 0},
			"warn_pct":         percentSchema(),
			"critical_pct":     percentSchema(),
		},
	}
}

func pricingSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"input_per_million":       nonNegativeNumberSchema(),
			"output_per_million":      nonNegativeNumberSchema(),
			"cache_read_per_million":  nonNegativeNumberSchema(),
			"cache_write_per_million": nonNegativeNumberSchema(),
		},
	}
}

func stringArraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "string"},
	}
}

func nonNegativeNumberSchema() map[string]any {
	return map[string]any{"type": "number", "minimum": 0}
}

func percentSchema() map[string]any {
	return map[string]any{"type": "number", "minimum": 0, "maximum": 100}
}
