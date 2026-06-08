package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nashory/agent-cockpit/internal/config"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectAcrossSources(t *testing.T) {
	cdir := t.TempDir()
	xdir := t.TempDir()
	write(t, filepath.Join(cdir, "proj", "a.jsonl"),
		`{"type":"assistant","sessionId":"s1","message":{"model":"claude-opus-4-8","usage":{"output_tokens":50}}}`)
	write(t, filepath.Join(xdir, "b.jsonl"),
		`{"type":"session_meta","payload":{"id":"x1","model":"gpt-5-codex"}}`+"\n"+
			`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"output_tokens":80}}}}`)

	cfg := config.Config{Paths: config.Paths{Claude: []string{cdir}, Codex: []string{xdir}}}
	events, err := Collect(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	bySource := map[string]int{}
	for _, e := range events {
		bySource[e.Source]++
	}
	if bySource["claude"] != 1 || bySource["codex"] != 1 {
		t.Fatalf("expected one event per source, got %v", bySource)
	}
}

func TestAllRegistersThreeSources(t *testing.T) {
	names := map[string]bool{}
	for _, s := range All() {
		names[s.Name()] = true
	}
	for _, want := range []string{"claude", "codex", "gemini"} {
		if !names[want] {
			t.Errorf("source registry missing %q", want)
		}
	}
}
