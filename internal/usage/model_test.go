package usage

import (
	"testing"
	"time"
)

func TestSummarizeAndGroupBy(t *testing.T) {
	events := []Event{
		{Source: "claude", Model: "claude-sonnet-4", Input: 10, Output: 20, CacheRead: 5},
		{Source: "codex", Model: "gpt-5-codex", Input: 3, Output: 7, Reasoning: 2},
	}

	totals := Summarize(events)
	if totals.Events != 2 || totals.Total != 47 || totals.Input != 13 || totals.Output != 27 {
		t.Fatalf("unexpected totals: %+v", totals)
	}

	buckets := GroupBy(events, func(e Event) string { return e.Source })
	if len(buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(buckets))
	}
	if buckets[0].Key != "claude" || buckets[0].Totals.Total != 35 {
		t.Fatalf("unexpected first bucket: %+v", buckets[0])
	}
}

func TestComputeInsights(t *testing.T) {
	day := func(d, h int) time.Time {
		return time.Date(2026, 1, d, h, 0, 0, 0, time.UTC)
	}
	events := []Event{
		{Source: "claude", Model: "claude-opus-4-8", Project: "p1", SessionID: "s1",
			Input: 100, Output: 50, CacheRead: 300, Timestamp: day(1, 9)},
		{Source: "codex", Model: "gpt-5-codex", Project: "p2", SessionID: "s2",
			Input: 200, Output: 100, Reasoning: 40, Timestamp: day(3, 14)},
	}

	ins := ComputeInsights(events, nil)

	if ins.Sessions != 2 || ins.Projects != 2 || ins.Models != 2 || ins.Agents != 2 {
		t.Fatalf("diversity wrong: %+v", ins)
	}
	if ins.ActiveDays != 2 || ins.SpanDays != 3 {
		t.Fatalf("cadence span wrong: active=%d span=%d", ins.ActiveDays, ins.SpanDays)
	}
	// cache hit = 300 / (300 input + 300 cache) = 0.5
	if got := ins.CacheHitRate; got < 0.49 || got > 0.51 {
		t.Fatalf("cache hit rate = %v, want ~0.5", got)
	}
	// in:out = 300 input / 150 output = 2.0
	if got := ins.InputOutputRatio; got < 1.99 || got > 2.01 {
		t.Fatalf("in:out = %v, want ~2.0", got)
	}
	if ins.BusiestHour != 9 && ins.BusiestHour != 14 {
		t.Fatalf("busiest hour unexpected: %d", ins.BusiestHour)
	}
	// opus event drives premium share > 0 under default pricing.
	if ins.PremiumCostShare <= 0 {
		t.Fatalf("premium share should be > 0, got %v", ins.PremiumCostShare)
	}
}
