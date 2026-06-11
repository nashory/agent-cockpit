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
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	since, until, err := window(&options{days: 30}, time.UTC, now)
	if err != nil {
		t.Fatal(err)
	}
	if !until.IsZero() {
		t.Errorf("until should be zero when not set, got %v", until)
	}
	want := now.AddDate(0, 0, -30)
	if !since.Equal(want) {
		t.Errorf("since = %v, want %v", since, want)
	}
}

func TestWindowExplicitDates(t *testing.T) {
	since, until, err := window(&options{since: "2026-01-01", until: "2026-01-31"}, time.UTC, time.Time{})
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

func TestWindowUsesTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Fatal(err)
	}
	since, until, err := window(&options{since: "2026-06-01", until: "2026-06-01"}, loc, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if since.Location() != loc || until.Location() != loc {
		t.Fatalf("window locations = %v %v, want %v", since.Location(), until.Location(), loc)
	}
	if since.Hour() != 0 || until.Hour() != 23 {
		t.Fatalf("window day bounds = %v %v", since, until)
	}
}

func TestWindowSinceOverridesDays(t *testing.T) {
	since, _, err := window(&options{since: "2026-06-01", days: 30}, time.UTC, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !since.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("explicit since should win over days, got %v", since)
	}
}

func TestWindowRelativeSince(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		raw  string
		want time.Time
	}{
		{name: "days", raw: "7d", want: now.AddDate(0, 0, -7)},
		{name: "weeks", raw: "2w", want: now.AddDate(0, 0, -14)},
		{name: "duration", raw: "168h", want: now.Add(-168 * time.Hour)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			since, until, err := window(&options{since: tc.raw}, time.UTC, now)
			if err != nil {
				t.Fatal(err)
			}
			if !until.IsZero() {
				t.Errorf("until should be zero when not set, got %v", until)
			}
			if !since.Equal(tc.want) {
				t.Errorf("since = %v, want %v", since, tc.want)
			}
		})
	}
}

func TestWindowBadDates(t *testing.T) {
	if _, _, err := window(&options{since: "nope"}, time.UTC, time.Time{}); err == nil {
		t.Error("expected error for invalid --since")
	}
	if _, _, err := window(&options{since: "0d"}, time.UTC, time.Time{}); err == nil {
		t.Error("expected error for invalid relative --since")
	}
	if _, _, err := window(&options{until: "2026-13-99"}, time.UTC, time.Time{}); err == nil {
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
		days:      7,
		sources:   "claude,codex",
		project:   "agent-cockpit",
		model:     "opus",
		order:     "asc",
		breakdown: "project",
	}, "Europe/Zurich")
	if ctx.Range.Days != 7 {
		t.Fatalf("days = %d, want 7", ctx.Range.Days)
	}
	if ctx.Range.Timezone != "Europe/Zurich" {
		t.Fatalf("timezone = %q", ctx.Range.Timezone)
	}
	if ctx.Order != "asc" {
		t.Fatalf("order = %q", ctx.Order)
	}
	if ctx.Breakdown != "project" {
		t.Fatalf("breakdown = %q", ctx.Breakdown)
	}
	if got := strings.Join(ctx.Filters.Sources, ","); got != "claude,codex" {
		t.Fatalf("sources = %q", got)
	}
	if ctx.Filters.Project != "agent-cockpit" || ctx.Filters.Model != "opus" {
		t.Fatalf("filters = %+v", ctx.Filters)
	}

	ctx = buildUsageJSONContext(&options{days: 30, since: "2026-06-01", until: "2026-06-11"}, "")
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

	trends, ok = usageJSONRows(events, cfg, now, usageJSONContext{Report: "trends", Order: "desc", Range: usageJSONRange{Days: 2}}).([]trendJSONRow)
	if !ok {
		t.Fatalf("trend rows type = %T", trends)
	}
	if len(trends) != 2 || trends[0].Date != "2026-06-11" || trends[1].Date != "2026-06-10" {
		t.Fatalf("desc trend rows = %+v", trends)
	}

	speed, ok := usageJSONRows(events, cfg, now, usageJSONContext{Report: "speed"}).([]speedJSONRow)
	if !ok {
		t.Fatalf("speed rows type = %T", speed)
	}
	if len(speed) != 2 || speed[0].OutputTokens == 0 {
		t.Fatalf("speed rows = %+v", speed)
	}
}

func TestOrderOutputs(t *testing.T) {
	cfg := goldenConfig()
	opts := &options{order: "asc"}

	var out bytes.Buffer
	if err := writeCSV(&out, csvGoldenEvents(), reportOptions(cfg, opts), "daily"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[1], "2026-06-10,") || !strings.HasPrefix(lines[2], "2026-06-11,") {
		t.Fatalf("asc daily CSV order:\n%s", out.String())
	}

	out.Reset()
	ctx := buildUsageJSONContext(opts, "")
	ctx.Report = "agents"
	if err := writeUsageJSON(&out, csvGoldenEvents(), cfg, goldenNow(), ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"order": "asc"`) {
		t.Fatalf("JSON missing order:\n%s", out.String())
	}

	if err := validateOrder("sideways"); err == nil {
		t.Fatal("expected invalid order error")
	}
}

func TestBreakdownOutputs(t *testing.T) {
	cfg := goldenConfig()
	opts := &options{breakdown: "project"}

	var out bytes.Buffer
	ctx := buildUsageJSONContext(opts, "")
	ctx.Report = "report"
	if err := writeUsageJSON(&out, csvGoldenEvents(), cfg, goldenNow(), ctx); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, `"breakdown": "project"`) || !strings.Contains(got, `"name": "projects"`) {
		t.Fatalf("JSON missing project breakdown:\n%s", got)
	}
	if strings.Contains(got, `"name": "agents"`) || strings.Contains(got, `"name": "models"`) {
		t.Fatalf("JSON should narrow summary rows to requested breakdown:\n%s", got)
	}

	out.Reset()
	if err := writeCSV(&out, csvGoldenEvents(), reportOptions(cfg, opts), "daily"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "project,events,") {
		t.Fatalf("CSV breakdown should override daily aggregate:\n%s", out.String())
	}

	if err := validateBreakdown("team"); err == nil {
		t.Fatal("expected invalid breakdown error")
	}
}

func TestStatuslineJSONGolden(t *testing.T) {
	var out bytes.Buffer
	totals := usage.SummarizeWith(goldenEvents(), nil)
	if err := writeStatuslineJSON(&out, totals, "USD", nil, nil, goldenNow(), false, nil); err != nil {
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

func TestStatuslineClaudeContext(t *testing.T) {
	used := 8.0
	cost := 0.01234
	ctx, err := parseClaudeStatuslineInput([]byte(`{
		"cwd": "/tmp/project",
		"session_id": "s1",
		"session_name": "debug-session",
		"transcript_path": "/tmp/transcript.jsonl",
		"version": "2.1.153",
		"model": {"id": "claude-sonnet-4-5", "display_name": "Sonnet"},
		"workspace": {"current_dir": "/tmp/project/pkg", "project_dir": "/tmp/project"},
		"cost": {"total_cost_usd": 0.01234},
		"context_window": {
			"context_window_size": 200000,
			"used_percentage": 8,
			"remaining_percentage": 92,
			"total_input_tokens": 15500,
			"total_output_tokens": 1200
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if ctx.SessionID != "s1" || ctx.Model.DisplayName != "Sonnet" {
		t.Fatalf("unexpected parsed context: %+v", ctx)
	}
	if ctx.ContextWindow.UsedPercentage == nil || *ctx.ContextWindow.UsedPercentage != used {
		t.Fatalf("used percentage = %v, want %v", ctx.ContextWindow.UsedPercentage, used)
	}
	if ctx.Cost.TotalCostUSD == nil || *ctx.Cost.TotalCostUSD != cost {
		t.Fatalf("cost = %v, want %v", ctx.Cost.TotalCostUSD, cost)
	}

	events := append(goldenEvents(), usage.Event{
		Source:    "claude",
		SessionID: "other",
		Model:     "claude-opus-4-8",
		Input:     1000,
		Timestamp: goldenNow(),
	})
	opts := &options{configPath: emptyConfigPath(t), statusline: ctx}

	var out bytes.Buffer
	writeStatusline(&out, events, reportOptions(goldenConfig(), opts), opts)
	got := out.String()
	if !strings.Contains(got, "model Sonnet | ctx 8%") {
		t.Fatalf("statusline missing active context:\n%s", got)
	}
	if !strings.Contains(got, "tokens 180") {
		t.Fatalf("statusline should use matching active session events:\n%s", got)
	}

	opts.json = true
	out.Reset()
	writeStatusline(&out, events, reportOptions(goldenConfig(), opts), opts)
	got = out.String()
	if !strings.Contains(got, `"active"`) || !strings.Contains(got, `"model_name": "Sonnet"`) || !strings.Contains(got, `"context_used_percentage": 8`) {
		t.Fatalf("statusline JSON missing active context:\n%s", got)
	}
}

func TestNoCostOutputs(t *testing.T) {
	cfg := goldenConfig()
	opts := &options{noCost: true, days: 30}

	var out bytes.Buffer
	ctx := buildUsageJSONContext(opts, "")
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
