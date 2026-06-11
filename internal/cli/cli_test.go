package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/usage"
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
	// TOML literal (single-quoted) strings keep Windows backslash paths intact.
	mustWrite(t, cfgPath, "[paths]\n"+
		"claude = ['"+claudeDir+"']\n"+
		"codex = ['"+emptyDir+"']\n"+
		"gemini = ['"+emptyDir+"']\n")

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

func TestUsageJSONGolden(t *testing.T) {
	var out bytes.Buffer
	if err := writeUsageJSON(&out, goldenEvents(), goldenConfig(), goldenNow()); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "usage_json.golden", out.String())
}

func TestStatuslineJSONGolden(t *testing.T) {
	var out bytes.Buffer
	totals := usage.SummarizeWith(goldenEvents(), nil)
	if err := writeStatuslineJSON(&out, totals, "USD", nil, nil, goldenNow()); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "statusline_json.golden", out.String())
}

func TestCSVGoldens(t *testing.T) {
	for _, group := range []string{"daily", "session", "model", "project", "event"} {
		t.Run(group, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeCSV(&out, csvGoldenEvents(), reportOptions(goldenConfig()), group); err != nil {
				t.Fatal(err)
			}
			assertGolden(t, "export_"+group+".csv", out.String())
		})
	}
}

func TestStatuslineTextGoldens(t *testing.T) {
	cfgPath := emptyConfigPath(t)
	for _, tc := range []struct {
		name string
		opts options
	}{
		{name: "default", opts: options{configPath: cfgPath}},
		{name: "compact", opts: options{configPath: cfgPath, compact: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			writeStatusline(&out, goldenEvents(), reportOptions(goldenConfig()), &tc.opts)
			assertGolden(t, "statusline_"+tc.name+".txt", out.String())
		})
	}
}

func emptyConfigPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func goldenConfig() config.Config {
	return config.Config{
		Currency: "USD",
		Pricing:  map[string]usage.Pricing{},
	}
}

func goldenEvents() []usage.Event {
	return []usage.Event{{
		Source:      "claude",
		SessionID:   "s1",
		Project:     "proj",
		CWD:         "/repo",
		Model:       "unknown-model",
		Input:       100,
		Output:      50,
		CacheRead:   20,
		CacheCreate: 10,
		Reasoning:   5,
		Timestamp:   time.Date(2026, 6, 11, 8, 0, 0, 0, time.UTC),
	}}
}

func csvGoldenEvents() []usage.Event {
	return []usage.Event{
		goldenEvents()[0],
		{
			Source:    "codex",
			SessionID: "s2",
			Project:   "proj-b",
			CWD:       "/repo-b",
			Model:     "unknown-model-b",
			Input:     7,
			Output:    3,
			Reasoning: 2,
			Timestamp: time.Date(2026, 6, 10, 9, 30, 0, 0, time.UTC),
		},
	}
}

func goldenNow() time.Time {
	return time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	normalizedWant := strings.ReplaceAll(string(want), "\r\n", "\n")
	if got != normalizedWant {
		t.Fatalf("%s mismatch\nwant:\n%s\ngot:\n%s", name, want, got)
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
