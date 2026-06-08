package codex

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	lines := strings.Join([]string{
		`{"timestamp":"2026-01-02T03:04:05Z","type":"session_meta","payload":{"id":"sess-1","cwd":"/home/u/proj","model":"gpt-5-codex"}}`,
		`{"timestamp":"2026-01-02T03:05:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":300,"cached_input_tokens":40,"output_tokens":80,"reasoning_output_tokens":20}}}}`,
		`{"timestamp":"2026-01-02T03:06:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":0,"output_tokens":0,"cached_input_tokens":0,"reasoning_output_tokens":0}}}}`, // zero -> skip
		`{"type":"event_msg","payload":{"type":"agent_message"}}`, // not token_count -> skip
	}, "\n")

	events, err := Parse(strings.NewReader(lines), "/logs/x.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 token event, got %d", len(events))
	}
	e := events[0]
	if e.Source != "codex" || e.SessionID != "sess-1" || e.Model != "gpt-5-codex" {
		t.Fatalf("unexpected metadata from session_meta: %+v", e)
	}
	// input_tokens (300) includes cached_input_tokens (40); Input is the
	// non-cached remainder so cached is not double-charged at the input rate.
	if e.Input != 260 || e.CacheRead != 40 || e.Output != 80 || e.Reasoning != 20 {
		t.Fatalf("unexpected token counts: %+v", e)
	}
	if e.Project != "proj" {
		t.Fatalf("project = %q, want proj", e.Project)
	}
}

func TestParseModelFallback(t *testing.T) {
	// No session_meta model -> defaults to "codex".
	line := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"output_tokens":5}}}}`
	events, err := Parse(strings.NewReader(line), "/logs/dir/x.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Model != "codex" {
		t.Fatalf("expected model fallback to codex, got %+v", events)
	}
}
