package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/usage"
)

func sample() []usage.Event {
	t0 := time.Now()
	return []usage.Event{
		{Source: "claude", Model: "claude-opus-4-8", Input: 100, Output: 50, Timestamp: t0},
		{Source: "codex", Model: "gpt-5-codex", Input: 200, Output: 80, Timestamp: t0.Add(-2 * time.Second)},
	}
}

func TestOverviewWritesTotals(t *testing.T) {
	var b bytes.Buffer
	Overview(&b, "Snapshot", sample(), Options{Currency: "USD"})
	s := b.String()
	for _, want := range []string{"Snapshot", "Events: 2", "Tokens:", "claude", "codex"} {
		if !strings.Contains(s, want) {
			t.Errorf("Overview output missing %q\n%s", want, s)
		}
	}
}

func TestBucketsRespectsLimit(t *testing.T) {
	var b bytes.Buffer
	buckets := usage.GroupBy(sample(), func(e usage.Event) string { return e.Source })
	Buckets(&b, "Agents", buckets, 1, Options{Currency: "USD"})
	s := b.String()
	// Only the top bucket (codex, more tokens) should appear under limit 1.
	if !strings.Contains(s, "codex") {
		t.Errorf("top bucket codex missing:\n%s", s)
	}
	if strings.Contains(s, "claude") {
		t.Errorf("limit 1 should omit the second bucket:\n%s", s)
	}
}

func TestSpeedAndTrendSmoke(t *testing.T) {
	var b bytes.Buffer
	Speed(&b, sample(), 10)
	if !strings.Contains(b.String(), "Observed Speed") {
		t.Errorf("Speed output unexpected:\n%s", b.String())
	}
	b.Reset()
	Trend(&b, sample(), 7, Options{Currency: "USD"})
	if b.Len() == 0 {
		t.Error("Trend produced no output")
	}
}

func TestFormatInt(t *testing.T) {
	cases := map[int64]string{0: "0", 12: "12", 1234: "1,234", 1234567: "1,234,567"}
	for n, want := range cases {
		if got := formatInt(n); got != want {
			t.Errorf("formatInt(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestSVGReport(t *testing.T) {
	events := []usage.Event{
		{Source: "claude", Model: "claude-opus-4-8", Input: 1000, Output: 200, Timestamp: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)},
		{Source: "codex", Model: "gpt-5-codex", Input: 500, Output: 100, Timestamp: time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)},
	}
	var b strings.Builder
	SVG(&b, "Usage summary", events, Options{Currency: "USD"})
	out := b.String()
	for _, want := range []string{"<svg", "</svg>", "agent-cockpit", "TOKENS", "TOP MODELS", "~"} {
		if !strings.Contains(out, want) {
			t.Errorf("SVG output missing %q", want)
		}
	}
	// XML-escapes model/text safely (no raw unescaped ampersand in a way that breaks).
	if strings.Count(out, "<svg") != 1 {
		t.Errorf("expected exactly one <svg root")
	}
}
