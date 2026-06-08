package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

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

// parse turns each non-"bad" line of a file into one event tagged with the line.
func parse(path string) ([]usage.Event, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []usage.Event
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		if line == "BAD" {
			return nil, errors.New("bad file")
		}
		out = append(out, usage.Event{Model: line, Input: 1})
	}
	return out, nil
}

func match(path string) bool { return strings.HasSuffix(path, ".jsonl") }

func TestParallelCollectsAll(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.jsonl"), "x\ny")
	write(t, filepath.Join(root, "sub", "b.jsonl"), "z")
	write(t, filepath.Join(root, "sub", "ignore.txt"), "nope")

	events, err := Parallel(context.Background(), []string{root}, match, parse)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(events))
	for _, e := range events {
		got = append(got, e.Model)
	}
	sort.Strings(got)
	want := []string{"x", "y", "z"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("collected %v, want %v", got, want)
	}
}

func TestParallelSkipsParseErrors(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "good.jsonl"), "ok")
	write(t, filepath.Join(root, "bad.jsonl"), "BAD")

	events, err := Parallel(context.Background(), []string{root}, match, parse)
	if err != nil {
		t.Fatalf("a single corrupt file must not fail the scan: %v", err)
	}
	if len(events) != 1 || events[0].Model != "ok" {
		t.Fatalf("got %v, want one 'ok' event", events)
	}
}

func TestParallelKeepsPartialOnError(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "f.jsonl"), "x\ny\nBOOM\nz")

	// A parser that yields events until it hits a fatal line (mimics a scanner
	// error mid-file): it returns what it gathered plus an error.
	partial := func(path string) ([]usage.Event, error) {
		b, _ := os.ReadFile(path)
		var out []usage.Event
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if line == "BOOM" {
				return out, errors.New("scanner error mid-file")
			}
			out = append(out, usage.Event{Model: line})
		}
		return out, nil
	}

	events, err := Parallel(context.Background(), []string{root}, match, partial)
	if err != nil {
		t.Fatalf("a mid-file error must not fail the scan: %v", err)
	}
	// The events parsed before the error (x, y) must survive.
	if len(events) != 2 {
		t.Fatalf("expected 2 partial events kept, got %d (%v)", len(events), events)
	}
}

func TestParallelMissingRoot(t *testing.T) {
	events, err := Parallel(context.Background(), []string{filepath.Join(t.TempDir(), "nope")}, match, parse)
	if err != nil {
		t.Fatalf("missing root should be skipped, got %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no events, got %d", len(events))
	}
}

func TestParallelContextCancelled(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		write(t, filepath.Join(root, "f", string(rune('a'+i))+".jsonl"), "x")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Parallel(ctx, []string{root}, match, parse)
	if err == nil {
		t.Fatal("expected ctx.Err() when context is cancelled")
	}
}
