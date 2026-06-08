package claude

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	lines := strings.Join([]string{
		`{"type":"assistant","timestamp":"2026-01-02T03:04:05Z","sessionId":"s1","cwd":"/home/u/proj","message":{"model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":200,"cache_creation_input_tokens":10}}}`,
		`{"type":"assistant","message":{"model":"claude-opus-4-8","usage":{"input_tokens":0,"output_tokens":0}}}`, // zero usage -> skipped
		`not json at all`, // malformed -> skipped
	}, "\n")

	events, err := Parse(strings.NewReader(lines), "/logs/proj/x.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (zero-usage and malformed skipped), got %d", len(events))
	}
	e := events[0]
	if e.Source != "claude" || e.SessionID != "s1" || e.Model != "claude-opus-4-8" {
		t.Fatalf("unexpected event metadata: %+v", e)
	}
	if e.Input != 100 || e.Output != 50 || e.CacheRead != 200 || e.CacheCreate != 10 {
		t.Fatalf("unexpected token counts: %+v", e)
	}
	if e.Project != "proj" {
		t.Fatalf("project = %q, want proj (from cwd basename)", e.Project)
	}
}

func TestProjectNameFallsBackToPath(t *testing.T) {
	// No cwd -> derive from the parent directory name.
	line := `{"type":"assistant","message":{"model":"m","usage":{"output_tokens":5}}}`
	events, err := Parse(strings.NewReader(line), "/logs/-home-u-myrepo/sess.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Project != "home-u-myrepo" {
		t.Fatalf("project = %q, want home-u-myrepo", events[0].Project)
	}
}
