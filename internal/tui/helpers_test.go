package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

func TestCompact(t *testing.T) {
	cases := map[int64]string{
		999:               "999",
		1_500:             "1.5K",
		1_500_000:         "1.5M",
		2_000_000_000:     "2.00B",
		3_000_000_000_000: "3.00T",
	}
	for n, want := range cases {
		if got := compact(n); got != want {
			t.Errorf("compact(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestGaugeWidthAndClamp(t *testing.T) {
	for _, ratio := range []float64{-1, 0, 0.5, 1, 2} {
		if w := lipgloss.Width(gauge(ratio, 10)); w != 10 {
			t.Errorf("gauge(%v,10) width = %d, want 10", ratio, w)
		}
	}
	if gauge(0.5, 0) != "" {
		// width<1 clamps to 1 cell; just ensure it does not panic / overflow
		if w := lipgloss.Width(gauge(0.5, 0)); w > 1 {
			t.Errorf("gauge with width 0 produced width %d", w)
		}
	}
}

func TestContribLevel(t *testing.T) {
	if contribLevel(0, 100) != 0 {
		t.Error("zero tokens should be level 0")
	}
	if l := contribLevel(100, 100); l != 4 {
		t.Errorf("max tokens level = %d, want 4", l)
	}
	if l := contribLevel(1, 100); l < 1 || l > 4 {
		t.Errorf("nonzero tokens level = %d, want 1..4", l)
	}
}

func TestAxisLineDeterministicNoOverlap(t *testing.T) {
	// "AB" occupies cols 0-1; the label at col 1 would overlap and is skipped.
	got := axisLine(10, map[int]string{0: "AB", 1: "CD"})
	if got != "AB        " {
		t.Fatalf("axisLine overlap handling = %q", got)
	}
	// Non-overlapping labels both render.
	got = axisLine(10, map[int]string{0: "A", 5: "B"})
	if got != "A    B    " {
		t.Fatalf("axisLine = %q", got)
	}
}

func TestFmtScale(t *testing.T) {
	cases := map[float64]string{0.05: "0.05", 5: "5.0", 50: "50"}
	for v, want := range cases {
		if got := fmtScale(v); got != want {
			t.Errorf("fmtScale(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestClampCursor(t *testing.T) {
	if clampCursor(-5) != 0 {
		t.Error("negative cursor should clamp to 0")
	}
	if clampCursor(10) != 10 {
		t.Error("in-range cursor should pass through")
	}
	if clampCursor(1<<30) != calMaxCursor {
		t.Error("large cursor should clamp to calMaxCursor")
	}
}

func TestSpeedRowsSortedByThroughput(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []usage.Event{
		{Source: "claude", Model: "opus", Output: 100, Timestamp: t0},
		{Source: "claude", Model: "opus", Output: 100, Timestamp: t0.Add(10 * time.Second)}, // 200/10s = 20 t/s
		{Source: "codex", Model: "gpt", Output: 100, Reasoning: 40, Timestamp: t0},          // reasoning ⊆ output
		{Source: "codex", Model: "gpt", Timestamp: t0.Add(20 * time.Second)},                // 100/20s = 5 t/s
	}
	rows := speedRows(events)
	if len(rows) != 2 {
		t.Fatalf("expected 2 lanes, got %d", len(rows))
	}
	if rows[0].key != "claude / opus" || rows[0].tps != 20 {
		t.Fatalf("fastest lane = %q @ %v t/s, want claude/opus @ 20", rows[0].key, rows[0].tps)
	}
	if rows[1].tps != 5 {
		t.Fatalf("slow lane tps = %v, want 5", rows[1].tps)
	}
}

func TestDailySeriesBuckets(t *testing.T) {
	now := time.Now()
	events := []usage.Event{
		{Output: 10, Timestamp: now},                    // today
		{Output: 5, Timestamp: now.AddDate(0, 0, -1)},   // yesterday
		{Output: 1, Timestamp: now.AddDate(0, 0, -100)}, // out of 7-day window
	}
	pts := dailySeries(events, 7)
	if len(pts) != 7 {
		t.Fatalf("expected 7 buckets, got %d", len(pts))
	}
	if pts[6].Value != 10 {
		t.Errorf("today bucket = %v, want 10", pts[6].Value)
	}
	if pts[5].Value != 5 {
		t.Errorf("yesterday bucket = %v, want 5", pts[5].Value)
	}
}

func TestTruncateASCIIUnchanged(t *testing.T) {
	if strings.Contains(truncate("short", 10), "…") {
		t.Error("short strings should not be truncated")
	}
}

func TestDailyThroughput(t *testing.T) {
	// Two events on the same UTC day, 2h apart, 3600 output each:
	// 7200 output / 7200 seconds = 1.0 tokens/sec on that day.
	base := time.Now().Truncate(24 * time.Hour).Add(9 * time.Hour)
	events := []usage.Event{
		{Output: 3600, Timestamp: base},
		{Output: 3600, Timestamp: base.Add(2 * time.Hour)},
	}
	pts := dailyThroughput(events, 30)
	last := pts[len(pts)-1].Value
	if last < 0.99 || last > 1.01 {
		t.Fatalf("throughput = %v t/s, want ~1.0", last)
	}
	// A single-event day has no measurable span -> 0.
	one := dailyThroughput([]usage.Event{{Output: 5000, Timestamp: base}}, 30)
	if v := one[len(one)-1].Value; v != 0 {
		t.Fatalf("single-event throughput = %v, want 0", v)
	}
}

func TestDailyVelocity(t *testing.T) {
	// Day-over-day change in total tokens (Output only, so Total == Output):
	// day-2 = 1000, day-1 = 3000, day0 = 2000.
	day := func(off int) time.Time {
		return time.Now().Truncate(24*time.Hour).Add(12*time.Hour).AddDate(0, 0, -off)
	}
	events := []usage.Event{
		{Output: 1000, Timestamp: day(2)},
		{Output: 3000, Timestamp: day(1)},
		{Output: 2000, Timestamp: day(0)},
	}
	pts := dailyVelocity(events, 30)
	n := len(pts)
	if got := pts[n-1].Value; got != -1000 { // 2000 - 3000
		t.Fatalf("velocity today = %v, want -1000", got)
	}
	if got := pts[n-2].Value; got != 2000 { // 3000 - 1000
		t.Fatalf("velocity yesterday = %v, want 2000", got)
	}
}
