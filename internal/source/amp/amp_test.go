package amp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
)

func TestParseLedgerEventsWithCacheTokens(t *testing.T) {
	body := `{
		"id":"thread-a",
		"usageLedger":{"events":[{
			"id":"event-a",
			"timestamp":"2026-01-02T00:00:00.000Z",
			"model":"gpt-5",
			"toMessageId":123,
			"tokens":{"input":100,"output":20,"total":150}
		}]},
		"messages":[{"role":"assistant","messageId":123,"usage":{
			"cacheCreationInputTokens":7,
			"cacheReadInputTokens":8
		}}]
	}`

	events, err := Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	event := events[0]
	if event.Source != "amp" || event.SessionID != "thread-a" || event.Model != "gpt-5" {
		t.Fatalf("unexpected event identity: %+v", event)
	}
	if event.Input != 100 || event.Output != 35 || event.CacheCreate != 7 || event.CacheRead != 8 {
		t.Fatalf("unexpected token mapping: %+v", event)
	}
	want := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if !event.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", event.Timestamp, want)
	}
}

func TestParseMessageUsageWhenLedgerMissing(t *testing.T) {
	body := `{
		"id":"T-thread-a",
		"messages":[
			{"role":"user","content":"hi"},
			{"role":"assistant","usage":{
				"model":"claude-haiku-4-5-20251001",
				"inputTokens":10,
				"outputTokens":178,
				"cacheCreationInputTokens":986,
				"cacheReadInputTokens":11372,
				"timestamp":"2026-01-19T11:42:10.652Z"
			}},
			{"role":"assistant","usage":{
				"model":"claude-haiku-4-5-20251001",
				"totalTokens":345,
				"timestamp":"2026-01-19T11:43:00.000Z"
			}}
		]
	}`

	events, err := Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want 2", events)
	}
	if events[0].Input != 10 || events[0].Output != 178 || events[0].CacheCreate != 986 || events[0].CacheRead != 11372 {
		t.Fatalf("unexpected first event: %+v", events[0])
	}
	if events[1].Output != 345 || events[1].TotalTokens() != 345 {
		t.Fatalf("total fallback not mapped: %+v", events[1])
	}
}

func TestCollectUsesAmpDataDirAndThreadsOnly(t *testing.T) {
	dir := t.TempDir()
	threads := filepath.Join(dir, "threads")
	if err := os.MkdirAll(threads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(threads, "T-a.json"), []byte(`{"id":"T-a","messages":[{"role":"assistant","usage":{"model":"gpt-5","inputTokens":1,"timestamp":"2026-01-02T00:00:00Z"}}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "not-thread.json"), []byte(`{"id":"bad","messages":[{"role":"assistant","usage":{"model":"gpt-5","inputTokens":99,"timestamp":"2026-01-02T00:00:00Z"}}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AMP_DATA_DIR", dir)
	events, err := (Source{}).Collect(context.Background(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].SessionID != "T-a" || events[0].Input != 1 {
		t.Fatalf("unexpected collected events: %+v", events)
	}
}

func TestParseSkipsMalformedAndEmptyUsage(t *testing.T) {
	events, err := Parse(strings.NewReader(`{"id":"T-a","messages":[{"role":"assistant","usage":{"model":"gpt-5","timestamp":"2026-01-02T00:00:00Z"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("zero-token message should be skipped: %+v", events)
	}
}
