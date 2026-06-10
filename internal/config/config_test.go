package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExpandPaths(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := expandPaths([]string{"~/x", "~", "", "/abs/y"})
	want := []string{filepath.Join(home, "x"), home, filepath.Clean("/abs/y")}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v (empty entry should be dropped)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expandPaths[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRefreshDuration(t *testing.T) {
	if d := (Config{RefreshInterval: "2s"}).RefreshDuration(); d != 2*time.Second {
		t.Errorf("valid duration = %v, want 2s", d)
	}
	if d := (Config{RefreshInterval: "garbage"}).RefreshDuration(); d != 3*time.Second {
		t.Errorf("invalid duration should fall back to 3s, got %v", d)
	}
	if d := (Config{RefreshInterval: "0s"}).RefreshDuration(); d != 3*time.Second {
		t.Errorf("zero duration should fall back to 3s, got %v", d)
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if cfg.Currency != "USD" || cfg.RefreshInterval != "3s" || len(cfg.Paths.Claude) == 0 {
		t.Fatalf("missing config should yield defaults, got %+v", cfg)
	}
}

func TestLoadOverridesAndExpands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `currency = "EUR"
refresh_interval = "5s"

[budget]
daily_usd = 12.5
warn_pct = 70

[limits]
claude_5h_tokens = 100000

[paths]
claude = ["~/.claude/projects"]

[pricing."claude-opus"]
input_per_million = 15
output_per_million = 75
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Currency != "EUR" || cfg.RefreshInterval != "5s" {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
	if cfg.Budget.DailyUSD != 12.5 || cfg.Budget.WarnPct != 70 {
		t.Fatalf("budget overrides not applied: %+v", cfg.Budget)
	}
	if cfg.Limits.Claude5HTokens != 100000 {
		t.Fatalf("limit overrides not applied: %+v", cfg.Limits)
	}
	home, _ := os.UserHomeDir()
	if want := filepath.Join(home, ".claude", "projects"); cfg.Paths.Claude[0] != want {
		t.Fatalf("claude path = %q, want expanded %q", cfg.Paths.Claude[0], want)
	}
	if p, ok := cfg.Pricing["claude-opus"]; !ok || p.InputPerMillion != 15 {
		t.Fatalf("pricing override missing: %+v", cfg.Pricing)
	}
}
