package usage

import "testing"

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
