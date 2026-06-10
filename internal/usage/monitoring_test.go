package usage

import (
	"testing"
	"time"
)

func TestBudgetStatuses(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	events := []Event{
		{Model: "claude-sonnet", Input: 1_000_000, Timestamp: now.Add(-time.Hour)},
		{Model: "claude-sonnet", Input: 1_000_000, Timestamp: now.AddDate(0, 0, -2)},
	}
	st := BudgetStatuses(events, PriceBook{"claude-sonnet": {InputPerMillion: 3}}, Budget{DailyUSD: 5, WeeklyUSD: 10, WarnPct: 50, CriticalPct: 90}, now)
	if len(st) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(st))
	}
	if st[0].Name != "daily budget" || st[0].Level != "warn" {
		t.Fatalf("daily status = %+v", st[0])
	}
	if st[1].Name != "7d budget" || st[1].Level != "warn" {
		t.Fatalf("weekly status = %+v", st[1])
	}
}

func TestClaudeLimitStatuses(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	events := []Event{
		{Source: "claude", Model: "claude-sonnet", Input: 50, Output: 50, Timestamp: now.Add(-time.Hour)},
		{Source: "codex", Model: "gpt-5", Input: 10, Output: 10, Timestamp: now.Add(-time.Hour)},
	}
	st := ClaudeLimitStatuses(events, nil, Limits{Claude5HTokens: 100, Claude7DTokens: 200, WarnPct: 80, CriticalPct: 95}, now)
	if len(st) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(st))
	}
	if st[0].Name != "claude 5h" || st[0].Level != "critical" || st[0].Ratio != 1 {
		t.Fatalf("5h status = %+v", st[0])
	}
	if st[1].Name != "claude 7d" || st[1].Level != "ok" {
		t.Fatalf("7d status = %+v", st[1])
	}
}

func TestResolvePricingSource(t *testing.T) {
	p, src := ResolvePricing("my-claude-sonnet-special", PriceBook{"sonnet-special": {InputPerMillion: 9}})
	if p.InputPerMillion != 9 || src != "config:sonnet-special" {
		t.Fatalf("config pricing = %+v / %s", p, src)
	}
	if _, src := ResolvePricing("claude-sonnet-4-5", nil); src != "vendored" && src != "fallback" {
		t.Fatalf("unexpected pricing source %q", src)
	}
	if VendoredPricingCount() == 0 {
		t.Fatal("expected vendored pricing data")
	}
}
