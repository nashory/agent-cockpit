package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/usage"
)

func sample() []usage.Event {
	t0 := time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)
	return []usage.Event{
		{Source: "claude", SessionID: "session-alpha", Project: "proj-a", Model: "unknown-a", Input: 100, Output: 50, Timestamp: t0},
		{Source: "codex", SessionID: "session-beta", Project: "proj-b", Model: "unknown-b", Input: 200, Output: 80, Timestamp: t0.Add(-2 * time.Second)},
	}
}

func speedSample() []usage.Event {
	return []usage.Event{
		{Source: "codex", Model: "unknown-b", Output: 20, Timestamp: time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)},
		{Source: "codex", Model: "unknown-b", Output: 40, Timestamp: time.Date(2026, 6, 11, 9, 0, 10, 0, time.UTC)},
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

func TestOverviewGolden(t *testing.T) {
	var b bytes.Buffer
	Overview(&b, "Snapshot", sample(), Options{Currency: "USD"})
	assertGolden(t, "overview.txt", b.String())
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

func TestBucketsGolden(t *testing.T) {
	var b bytes.Buffer
	buckets := usage.GroupBy(sample(), func(e usage.Event) string { return e.Source })
	Buckets(&b, "Agents", buckets, 0, Options{Currency: "USD"})
	assertGolden(t, "buckets.txt", b.String())
}

func TestSessionsGolden(t *testing.T) {
	var b bytes.Buffer
	Sessions(&b, sample(), 10, Options{Currency: "USD"})
	assertGolden(t, "sessions.txt", b.String())
}

func TestSpeedGolden(t *testing.T) {
	var b bytes.Buffer
	Speed(&b, speedSample(), 10)
	assertGolden(t, "speed.txt", b.String())
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

func TestSVGReportNoCost(t *testing.T) {
	var b strings.Builder
	SVG(&b, "Usage summary", sample(), Options{Currency: "USD", NoCost: true})
	out := b.String()
	for _, leaked := range []string{"COST", "~", "LiteLLM pricing"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("no-cost SVG leaked %q:\n%s", leaked, out)
		}
	}
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
