package cli

import (
	"io"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/usage"
)

func BenchmarkStatuslineRender(b *testing.B) {
	events := benchmarkEvents(10_000)
	cfg := goldenConfig()
	cfg.Limits.Claude5HTokens = 2_000_000
	used := 42.0
	remaining := 58.0
	ctx := &claudeStatuslineContext{SessionID: "session-1"}
	ctx.Model.ID = "claude-sonnet-4-5"
	ctx.Model.DisplayName = "Sonnet"
	ctx.ContextWindow.UsedPercentage = &used
	ctx.ContextWindow.RemainingPercent = &remaining

	tests := []struct {
		name string
		opts options
	}{
		{name: "default", opts: options{configPath: emptyConfigPath(b)}},
		{name: "json", opts: options{configPath: emptyConfigPath(b), json: true}},
		{name: "format", opts: options{
			configPath:   emptyConfigPath(b),
			statusline:   ctx,
			statusFormat: "{{model}} {{context}} {{tokens_compact}} {{block_left}}",
		}},
	}
	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			opts := tc.opts
			ro := reportOptions(cfg, &opts)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				writeStatusline(io.Discard, events, ro, &opts)
			}
		})
	}
}

func benchmarkEvents(n int) []usage.Event {
	out := make([]usage.Event, 0, n)
	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	models := []string{"claude-sonnet-4-5", "claude-opus-4-8", "gpt-5-codex", "gemini-2.5-pro"}
	sources := []string{"claude", "claude", "codex", "gemini"}
	for i := 0; i < n; i++ {
		idx := i % len(models)
		out = append(out, usage.Event{
			Source:      sources[idx],
			SessionID:   "session-" + string(rune('0'+i%10)),
			Project:     "agent-cockpit",
			CWD:         "/tmp/agent-cockpit",
			Model:       models[idx],
			Input:       int64(100 + i%50),
			Output:      int64(25 + i%20),
			CacheRead:   int64(i % 10),
			CacheCreate: int64(i % 7),
			Reasoning:   int64(i % 5),
			Timestamp:   base.Add(-time.Duration(i%720) * time.Minute),
		})
	}
	return out
}
