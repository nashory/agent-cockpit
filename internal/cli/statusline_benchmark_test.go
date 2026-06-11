package cli

import (
	"io"
	"testing"

	"github.com/nashory/agent-cockpit/internal/benchdata"
)

func BenchmarkStatuslineRender(b *testing.B) {
	events := benchdata.Events(10_000)
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
