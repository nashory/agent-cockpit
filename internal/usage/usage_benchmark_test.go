package usage_test

import (
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/benchdata"
	"github.com/nashory/agent-cockpit/internal/usage"
)

func BenchmarkUsageAggregation(b *testing.B) {
	events := benchdata.Events(100_000)
	prices := usage.PriceBook{
		"claude-sonnet-4-5": {InputPerMillion: 3, OutputPerMillion: 15, CacheReadPerMillion: 0.3, CacheWritePerMillion: 3.75},
		"claude-opus-4-8":   {InputPerMillion: 5, OutputPerMillion: 25, CacheReadPerMillion: 0.5, CacheWritePerMillion: 6.25},
		"gpt-5-codex":       {InputPerMillion: 1.25, OutputPerMillion: 10, CacheReadPerMillion: 0.125},
		"gemini-2.5-pro":    {InputPerMillion: 1.25, OutputPerMillion: 10},
	}
	since := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		run  func()
	}{
		{name: "summarize_tokens", run: func() { _ = usage.SummarizeTokens(events) }},
		{name: "summarize_with_cost", run: func() { _ = usage.SummarizeWith(events, prices) }},
		{name: "group_by_model", run: func() { _ = usage.GroupByWith(events, prices, func(e usage.Event) string { return e.Model }) }},
		{name: "filter_recent", run: func() { _ = usage.Filter(events, since, time.Time{}, []string{"claude", "codex"}, "", "") }},
		{name: "session_blocks", run: func() { _ = usage.SessionBlocks(events, prices, usage.DefaultBlockWindow) }},
		{name: "insights", run: func() { _ = usage.ComputeInsights(events, prices) }},
	}
	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tc.run()
			}
		})
	}
}
