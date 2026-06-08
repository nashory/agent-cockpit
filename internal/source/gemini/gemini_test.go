package gemini

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj", "chats")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "session-1.json")
	body := `{
      "sessionId": "g1",
      "startTime": "2026-01-02T00:00:00Z",
      "lastUpdated": "2026-01-02T01:00:00Z",
      "messages": [
        {"timestamp":"2026-01-02T00:30:00Z","type":"gemini","model":"gemini-2.5-pro","tokens":{"input":100,"output":40,"cached":10,"thoughts":5,"tool":2}},
        {"type":"user","model":"","tokens":{"input":0,"output":0,"cached":0,"thoughts":0,"tool":0}}
      ]
    }`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (empty message skipped), got %d", len(events))
	}
	e := events[0]
	if e.Source != "gemini" || e.SessionID != "g1" || e.Model != "gemini-2.5-pro" {
		t.Fatalf("unexpected metadata: %+v", e)
	}
	// input (100) includes cached (10), so Input is the non-cached remainder (90).
	// Output folds in thoughts (5) and tool (2); reasoning surfaces thoughts.
	if e.Input != 90 || e.Output != 47 || e.CacheRead != 10 || e.Reasoning != 5 {
		t.Fatalf("unexpected token counts: %+v", e)
	}
	if e.Project != "proj" {
		t.Fatalf("project = %q, want proj", e.Project)
	}
}
