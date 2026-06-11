package qwen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
)

func TestParseAssistantUsageMetadata(t *testing.T) {
	body := strings.Join([]string{
		`{"type":"user","usageMetadata":{"promptTokenCount":999}}`,
		`{"type":"assistant","model":"qwen3-coder-plus","timestamp":"2026-02-23T14:24:56.857Z","sessionId":"session-json","usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":50,"thoughtsTokenCount":10,"cachedContentTokenCount":5}}`,
	}, "\n")

	events, err := Parse(strings.NewReader(body), "/tmp/.qwen/projects/myProject/chats/chat-a.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	event := events[0]
	if event.Source != "qwen" || event.SessionID != "session-json" || event.Project != "myProject" || event.Model != "qwen3-coder-plus" {
		t.Fatalf("unexpected identity: %+v", event)
	}
	if event.Input != 100 || event.Output != 60 || event.CacheRead != 5 || event.Reasoning != 10 {
		t.Fatalf("unexpected tokens: %+v", event)
	}
	want := time.Date(2026, 2, 23, 14, 24, 56, 857000000, time.UTC)
	if !event.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", event.Timestamp, want)
	}
}

func TestParseFallsBackToTotalTokens(t *testing.T) {
	body := `{"type":"assistant","timestamp":"2026-02-23T14:24:56Z","usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":5,"totalTokenCount":180}}`

	events, err := Parse(strings.NewReader(body), "/tmp/.qwen/projects/myProject/chats/chat-a.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	if events[0].Output != 75 || events[0].TotalTokens() != 180 {
		t.Fatalf("total fallback not mapped: %+v", events[0])
	}
	if events[0].SessionID != "myProject-chat-a" || events[0].Model != defaultModel {
		t.Fatalf("fallback identity not mapped: %+v", events[0])
	}
}

func TestCollectUsesQwenDataDirAndChatShape(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "projects", "myProject", "chats", "chat-a.jsonl")
	nested := filepath.Join(dir, "projects", "myProject", "not-chats", "chat-b.jsonl")
	other := filepath.Join(dir, "projects", "myProject", "chats", "notes.txt")
	for _, path := range []string{good, nested, other} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"type":"assistant","timestamp":"2026-02-23T14:24:56Z","usageMetadata":{"candidatesTokenCount":1}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(dataDirEnv, dir)

	events, err := (Source{}).Collect(context.Background(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want only projects/*/chats/*.jsonl", events)
	}
	if events[0].Project != "myProject" || events[0].Output != 1 {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestParseSkipsMalformedAndZeroTokenLines(t *testing.T) {
	body := strings.Join([]string{
		"not json",
		`{"type":"assistant","usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0,"cachedContentTokenCount":0,"thoughtsTokenCount":0}}`,
	}, "\n")

	events, err := Parse(strings.NewReader(body), "/tmp/.qwen/projects/myProject/chats/chat-a.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("zero-token lines should be skipped: %+v", events)
	}
}
