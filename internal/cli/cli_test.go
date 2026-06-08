package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowDaysDefault(t *testing.T) {
	since, until, err := window(&options{days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if !until.IsZero() {
		t.Errorf("until should be zero when not set, got %v", until)
	}
	want := time.Now().AddDate(0, 0, -30)
	if d := since.Sub(want); d < -time.Minute || d > time.Minute {
		t.Errorf("since = %v, want ~%v", since, want)
	}
}

func TestWindowExplicitDates(t *testing.T) {
	since, until, err := window(&options{since: "2026-01-01", until: "2026-01-31"})
	if err != nil {
		t.Fatal(err)
	}
	if !since.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("since = %v, want 2026-01-01", since)
	}
	// until is pushed to the end of the day.
	endOfDay := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)
	if until.Before(endOfDay) {
		t.Errorf("until = %v, want end of 2026-01-31", until)
	}
}

func TestWindowSinceOverridesDays(t *testing.T) {
	since, _, err := window(&options{since: "2026-06-01", days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if !since.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("explicit since should win over days, got %v", since)
	}
}

func TestWindowBadDates(t *testing.T) {
	if _, _, err := window(&options{since: "nope"}); err == nil {
		t.Error("expected error for invalid --since")
	}
	if _, _, err := window(&options{until: "2026-13-99"}); err == nil {
		t.Error("expected error for invalid --until")
	}
}

func TestLoadFiltersConfiguredSources(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, "claude")
	emptyDir := filepath.Join(dir, "empty")
	mustWrite(t, filepath.Join(claudeDir, "proj", "a.jsonl"),
		`{"type":"assistant","timestamp":"2026-06-01T00:00:00Z","sessionId":"s1","message":{"model":"claude-opus-4-8","usage":{"output_tokens":50}}}`)
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "config.toml")
	// Point every source at temp dirs so the test never reads real ~/.codex etc.
	mustWrite(t, cfgPath, `[paths]
claude = ["`+claudeDir+`"]
codex = ["`+emptyDir+`"]
gemini = ["`+emptyDir+`"]
`)

	events, cfg, err := load(context.Background(), &options{configPath: cfgPath, days: 36500})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Currency == "" {
		t.Error("expected config defaults to be populated")
	}
	if len(events) != 1 || events[0].Source != "claude" {
		t.Fatalf("expected one claude event, got %+v", events)
	}

	// Model filter excludes the event.
	events, _, err = load(context.Background(), &options{configPath: cfgPath, days: 36500, model: "gemini"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("model filter should exclude claude event, got %d", len(events))
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
