package source_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/source"
	_ "github.com/nashory/agent-cockpit/internal/source/builtin"
	"github.com/nashory/agent-cockpit/internal/usage"
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
	events, err := source.Collect(context.Background(), cfg)
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

func TestAllRegistersBuiltInSources(t *testing.T) {
	names := map[string]bool{}
	for _, s := range source.All() {
		names[s.Name()] = true
	}
	for _, want := range []string{"claude", "codex", "gemini", "opencode", "amp", "copilot", "kimi", "qwen", "codebuff", "kilo", "goose"} {
		if !names[want] {
			t.Errorf("source registry missing %q", want)
		}
	}
}

type testFileAdapter struct {
	root string
}

func (a testFileAdapter) Name() string { return "test-file" }

func (a testFileAdapter) Roots(config.Config) []string { return []string{a.root} }

func (testFileAdapter) Match(path string) bool {
	return filepath.Ext(path) == ".log"
}

func (testFileAdapter) Parse(path string, r io.Reader) ([]usage.Event, error) {
	if filepath.Base(path) == "bad.log" {
		return nil, os.ErrInvalid
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return []usage.Event{{
		Source:    "test-file",
		SessionID: string(body),
	}}, nil
}

func TestCollectFilesSkipsFileLocalErrors(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "ok.log"), "ok")
	write(t, filepath.Join(root, "bad.log"), "bad")
	write(t, filepath.Join(root, "ignored.txt"), "ignored")

	events, err := source.CollectFiles(context.Background(), config.Config{}, testFileAdapter{root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one parsed event, got %d", len(events))
	}
	if events[0].SessionID != "ok" {
		t.Fatalf("expected ok event, got %#v", events[0])
	}
}
