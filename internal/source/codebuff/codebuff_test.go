package codebuff

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
)

func TestParseAssistantMetadataUsage(t *testing.T) {
	body := `[
  {"role":"user","metadata":{"usage":{"inputTokens":999}}},
  {"id":"msg-1","variant":"assistant","timestamp":"2026-02-23T14:24:56.857Z","metadata":{"model":"claude-sonnet-4-5","usage":{"inputTokens":100,"outputTokens":50,"cacheReadInputTokens":5,"cacheCreationInputTokens":7}}}
]`

	events, err := Parse(strings.NewReader(body), "/tmp/.config/manicode/projects/myProject/chats/2026-02-23T14-24-56.857Z/chat-messages.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	event := events[0]
	if event.Source != "codebuff" || event.Project != "myProject" || event.SessionID != "manicode/myProject/2026-02-23T14-24-56.857Z/msg-1" || event.Model != "claude-sonnet-4-5" {
		t.Fatalf("unexpected identity: %+v", event)
	}
	if event.Input != 100 || event.Output != 50 || event.CacheRead != 5 || event.CacheCreate != 7 {
		t.Fatalf("unexpected tokens: %+v", event)
	}
	want := time.Date(2026, 2, 23, 14, 24, 56, 857000000, time.UTC)
	if !event.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", event.Timestamp, want)
	}
}

func TestParseRunStateFallbackAndTotalTokens(t *testing.T) {
	body := `[
  {"role":"assistant","metadata":{"runState":{"sessionState":{"mainAgentState":{"messageHistory":[
    {"role":"assistant","providerOptions":{"codebuff":{"model":"gpt-5","usage":{"totalTokens":123}}}}
  ]}}}}}
]`

	events, err := Parse(strings.NewReader(body), "/tmp/.config/manicode/projects/myProject/chats/2026-02-23T14-24-56Z/chat-messages.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	if events[0].Output != 123 || events[0].TotalTokens() != 123 || events[0].Model != "gpt-5" {
		t.Fatalf("runState fallback not mapped: %+v", events[0])
	}
}

func TestCollectUsesCodebuffDataDirAndChatShape(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "projects", "myProject", "chats", "chat-a", "chat-messages.json")
	nested := filepath.Join(dir, "projects", "myProject", "chat-messages.json")
	other := filepath.Join(dir, "projects", "myProject", "chats", "chat-a", "other.json")
	for _, path := range []string{good, nested, other} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`[{"role":"assistant","metadata":{"usage":{"outputTokens":1}}}]`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(dataDirEnv, dir)

	events, err := (Source{}).Collect(context.Background(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want only projects/*/chats/*/chat-messages.json", events)
	}
	if events[0].Project != "myProject" || events[0].Output != 1 {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestParseSkipsMalformedAndZeroTokenMessages(t *testing.T) {
	for _, body := range []string{
		`not json`,
		`[{"role":"assistant","metadata":{"usage":{"inputTokens":0}}}]`,
	} {
		events, err := Parse(strings.NewReader(body), "/tmp/.config/manicode/projects/myProject/chats/chat-a/chat-messages.json")
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 0 {
			t.Fatalf("zero-token lines should be skipped: %+v", events)
		}
	}
}
