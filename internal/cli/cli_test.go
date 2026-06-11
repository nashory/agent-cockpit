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
	if err := writeUsageJSON(&out, goldenEvents(), goldenConfig(), goldenNow(), usageJSONContext{}); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "usage_json.golden", out.String())
}

func TestBuildUsageJSONContext(t *testing.T) {
	ctx := buildUsageJSONContext(&options{
		days:    7,
		sources: "claude,codex",
		project: "agent-cockpit",
		model:   "opus",
	})
	if ctx.Range.Days != 7 {
		t.Fatalf("days = %d, want 7", ctx.Range.Days)
	}
	if got := strings.Join(ctx.Filters.Sources, ","); got != "claude,codex" {
		t.Fatalf("sources = %q", got)
	}
	if ctx.Filters.Project != "agent-cockpit" || ctx.Filters.Model != "opus" {
		t.Fatalf("filters = %+v", ctx.Filters)
	}

	ctx = buildUsageJSONContext(&options{days: 30, since: "2026-06-01", until: "2026-06-11"})
	if ctx.Range.Days != 0 || ctx.Range.Since != "2026-06-01" || ctx.Range.Until != "2026-06-11" {
		t.Fatalf("explicit range should suppress days, got %+v", ctx.Range)
	}
}

func TestUsageJSONRowsByReport(t *testing.T) {
	cfg := goldenConfig()
	events := csvGoldenEvents()
	now := goldenNow()

	summary, ok := usageJSONRows(events, cfg, now, usageJSONContext{Report: "today"}).([]summaryJSONSection)
	if !ok {
		t.Fatalf("summary rows type = %T", summary)
	}
	if len(summary) != 2 || summary[0].Name != "agents" || summary[1].Name != "models" {
		t.Fatalf("summary rows = %+v", summary)
	}

	agents, ok := usageJSONRows(events, cfg, now, usageJSONContext{Report: "agents"}).([]bucketJSONRow)
	if !ok {
		t.Fatalf("agents rows type = %T", agents)
	}
	if len(agents) != 2 || agents[0].Key == "" {
		t.Fatalf("agents rows = %+v", agents)
	}

	sessions, ok := usageJSONRows(events, cfg, now, usageJSONContext{Report: "sessions"}).([]bucketJSONRow)
	if !ok {
		t.Fatalf("sessions rows type = %T", sessions)
	}
	if sessions[0].Key != "proj / s1" && sessions[0].Key != "proj-b / s2" {
		t.Fatalf("sessions rows = %+v", sessions)
	}

	trends, ok := usageJSONRows(events, cfg, now, usageJSONContext{Report: "trends", Range: usageJSONRange{Days: 2}}).([]trendJSONRow)
	if !ok {
		t.Fatalf("trend rows type = %T", trends)
	}
	if len(trends) != 2 || trends[0].Date != "2026-06-10" || trends[1].Date != "2026-06-11" {
		t.Fatalf("trend rows = %+v", trends)
	}

	speed, ok := usageJSONRows(events, cfg, now, usageJSONContext{Report: "speed"}).([]speedJSONRow)
	if !ok {
		t.Fatalf("speed rows type = %T", speed)
	}
	if len(speed) != 2 || speed[0].OutputTokens == 0 {
		t.Fatalf("speed rows = %+v", speed)
	}
}

func TestStatuslineJSONGolden(t *testing.T) {
	var out bytes.Buffer
	totals := usage.SummarizeWith(goldenEvents(), nil)
	if err := writeStatuslineJSON(&out, totals, "USD", nil, nil, goldenNow(), false); err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "statusline_json.golden", out.String())
}

func TestCSVGoldens(t *testing.T) {
	for _, group := range []string{"daily", "session", "model", "project", "event"} {
		t.Run(group, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeCSV(&out, csvGoldenEvents(), reportOptions(goldenConfig(), nil), group); err != nil {
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
			writeStatusline(&out, goldenEvents(), reportOptions(goldenConfig(), &tc.opts), &tc.opts)
			assertGolden(t, "statusline_"+tc.name+".txt", out.String())
		})
	}
}

func TestNoCostOutputs(t *testing.T) {
	cfg := goldenConfig()
	opts := &options{noCost: true, days: 30}

	var out bytes.Buffer
	ctx := buildUsageJSONContext(opts)
	ctx.Report = "today"
	if err := writeUsageJSON(&out, goldenEvents(), cfg, goldenNow(), ctx); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, `"cost_mode": "disabled"`) {
		t.Fatalf("no-cost JSON missing disabled cost mode:\n%s", got)
	}
	if strings.Contains(got, "estimated_cost_usd") {
		t.Fatalf("no-cost JSON should omit estimated_cost_usd:\n%s", got)
	}

	out.Reset()
	writeStatusline(&out, goldenEvents(), reportOptions(cfg, opts), opts)
	if strings.Contains(out.String(), "cost") || strings.Contains(out.String(), "~") {
		t.Fatalf("no-cost statusline leaked cost: %s", out.String())
	}

	out.Reset()
	if err := writeCSV(&out, csvGoldenEvents(), reportOptions(cfg, opts), "daily"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "estimated_cost") || strings.Contains(out.String(), "currency") {
		t.Fatalf("no-cost CSV leaked cost columns:\n%s", out.String())
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
