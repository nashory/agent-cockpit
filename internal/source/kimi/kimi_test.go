package kimi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
)

func TestParseWireStatusUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions", "group", "session-a", "wire.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"model":"kimi-k2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"type":"metadata","protocol_version":"1.3"}`,
		`{"timestamp":1770983426.420942,"message":{"type":"TurnBegin","payload":{"user_input":"hello"}}}`,
		`{"timestamp":1770983427.123,"message":{"type":"StatusUpdate","payload":{"token_usage":{"input_other":100,"output":50,"input_cache_read":10,"input_cache_creation":20},"message_id":"msg-1"}}}`,
	}, "\n")

	events, err := Parse(strings.NewReader(body), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	event := events[0]
	if event.Source != "kimi" || event.SessionID != "session-a" || event.Project != "kimi" || event.Model != "kimi-k2" {
		t.Fatalf("unexpected identity: %+v", event)
	}
	if event.Input != 100 || event.Output != 50 || event.CacheRead != 10 || event.CacheCreate != 20 {
		t.Fatalf("unexpected tokens: %+v", event)
	}
	want := time.Date(2026, 2, 13, 11, 50, 27, 123000000, time.UTC)
	if !event.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", event.Timestamp, want)
	}
}

func TestParseFallsBackToTotalTokens(t *testing.T) {
	body := `{"timestamp":1770983427.123,"message":{"type":"StatusUpdate","payload":{"token_usage":{"total":432}}}}`

	events, err := Parse(strings.NewReader(body), "/tmp/.kimi/sessions/group/session-a/wire.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	if events[0].Output != 432 || events[0].TotalTokens() != 432 {
		t.Fatalf("total fallback not mapped: %+v", events[0])
	}
}

func TestCollectUsesKimiDataDirAndWireShape(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "sessions", "group", "session-a", "wire.jsonl")
	nested := filepath.Join(dir, "sessions", "nested", "path", "session-b", "wire.jsonl")
	other := filepath.Join(dir, "sessions", "group", "session-a", "other.jsonl")
	for _, path := range []string{good, nested, other} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"timestamp":1770983427,"message":{"type":"StatusUpdate","payload":{"token_usage":{"output":1}}}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(dataDirEnv, dir)

	events, err := (Source{}).Collect(context.Background(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want only sessions/group/session/wire.jsonl", events)
	}
	if events[0].SessionID != "session-a" || events[0].Output != 1 {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestParseSkipsMalformedAndZeroTokenLines(t *testing.T) {
	body := strings.Join([]string{
		"not json",
		`{"timestamp":1770983427,"message":{"type":"StatusUpdate","payload":{"token_usage":{"input_other":0,"output":0,"input_cache_read":0,"input_cache_creation":0}}}}`,
	}, "\n")
	events, err := Parse(strings.NewReader(body), "/tmp/.kimi/sessions/group/session-a/wire.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("zero-token lines should be skipped: %+v", events)
	}
}
