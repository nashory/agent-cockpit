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

func TestFilter(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 12, 0, 0, 0, time.UTC) }
	events := []Event{
		{Source: "claude", Project: "alpha", Model: "claude-opus-4-8", Timestamp: day(1)},
		{Source: "codex", Project: "beta", Model: "gpt-5-codex", Timestamp: day(5)},
		{Source: "gemini", Project: "alpha", Model: "gemini-2.5-pro", Timestamp: day(10)},
	}

	// Date window keeps only the middle event.
	got := Filter(append([]Event(nil), events...), day(3), day(7), nil, "", "")
	if len(got) != 1 || got[0].Source != "codex" {
		t.Fatalf("date filter = %+v, want only codex", got)
	}
	// Source filter (case-insensitive).
	got = Filter(append([]Event(nil), events...), time.Time{}, time.Time{}, []string{"CLAUDE", "gemini"}, "", "")
	if len(got) != 2 {
		t.Fatalf("source filter kept %d, want 2", len(got))
	}
	// Project substring.
	got = Filter(append([]Event(nil), events...), time.Time{}, time.Time{}, nil, "alpha", "")
	if len(got) != 2 {
		t.Fatalf("project filter kept %d, want 2", len(got))
	}
	// Model substring.
	got = Filter(append([]Event(nil), events...), time.Time{}, time.Time{}, nil, "", "opus")
	if len(got) != 1 || got[0].Model != "claude-opus-4-8" {
		t.Fatalf("model filter = %+v, want opus only", got)
	}
}

func TestLookupPricingLongestMatch(t *testing.T) {
	prices := PriceBook{
		"claude":      {InputPerMillion: 1},
		"claude-opus": {InputPerMillion: 99},
		"gpt-5":       {InputPerMillion: 2},
	}
	// Longest substring wins.
	if p := lookupPricing("claude-opus-4-8", prices); p.InputPerMillion != 99 {
		t.Fatalf("expected longest match (claude-opus), got %v", p.InputPerMillion)
	}
	// Unknown model falls back to built-in defaults (opus tier here).
	if p := lookupPricing("claude-opus-4-8", nil); p.InputPerMillion != 15 {
		t.Fatalf("default opus input rate = %v, want 15", p.InputPerMillion)
	}
}

func TestEstimateCostWith(t *testing.T) {
	e := Event{Model: "x", Input: 1_000_000, Output: 1_000_000}
	prices := PriceBook{"x": {InputPerMillion: 2, OutputPerMillion: 10}}
	if c := EstimateCostWith(e, prices); c != 12 {
		t.Fatalf("cost = %v, want 12", c)
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
