package copilot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
)

func TestParseChatSpan(t *testing.T) {
	body := `{"type":"metric","name":"ignored"}
{"type":"span","traceId":"trace-1","spanId":"span-1","name":"chat claude-sonnet-4","endTime":[1775934264,967317833],"attributes":{"gen_ai.operation.name":"chat","gen_ai.request.model":"claude-sonnet-4","gen_ai.response.model":"claude-sonnet-4","gen_ai.conversation.id":"conv-1","gen_ai.usage.input_tokens":19452,"gen_ai.usage.output_tokens":281,"gen_ai.usage.cache_read.input_tokens":123,"gen_ai.usage.cache_creation.input_tokens":25,"gen_ai.usage.reasoning.output_tokens":128}}`

	events, err := Parse(strings.NewReader(body), "missing.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	event := events[0]
	if event.Source != "copilot" || event.SessionID != "conv-1" || event.Model != "claude-sonnet-4" {
		t.Fatalf("unexpected event identity: %+v", event)
	}
	if event.Input != 19329 || event.Output != 409 || event.CacheRead != 123 || event.CacheCreate != 25 || event.Reasoning != 128 {
		t.Fatalf("unexpected token mapping: %+v", event)
	}
	want := time.Date(2026, 4, 11, 19, 4, 24, 967317833, time.UTC)
	if !event.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", event.Timestamp, want)
	}
}

func TestParseSuppressesLowerPriorityRecordsForSameResponse(t *testing.T) {
	body := `{"type":"span","traceId":"trace-dupe","spanId":"span-chat","name":"chat gpt-5","attributes":{"gen_ai.operation.name":"chat","gen_ai.response.model":"gpt-5","gen_ai.conversation.id":"conv-dupe","gen_ai.response.id":"resp-dupe","gen_ai.usage.input_tokens":100,"gen_ai.usage.output_tokens":30}}
{"hrTime":[1775934263,0],"attributes":{"event.name":"gen_ai.client.inference.operation.details","gen_ai.response.model":"gpt-5","gen_ai.conversation.id":"conv-dupe","gen_ai.response.id":"resp-dupe","gen_ai.usage.input_tokens":100,"gen_ai.usage.output_tokens":30}}`

	events, err := Parse(strings.NewReader(body), "missing.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want chat span only", events)
	}
	if events[0].SessionID != "conv-dupe" || events[0].Input != 100 {
		t.Fatalf("unexpected retained event: %+v", events[0])
	}
}

func TestCollectUsesConfiguredDirsAndExporterFile(t *testing.T) {
	dir := t.TempDir()
	otel := filepath.Join(dir, "otel")
	if err := os.MkdirAll(otel, 0o755); err != nil {
		t.Fatal(err)
	}
	dirFile := filepath.Join(otel, "copilot.jsonl")
	if err := os.WriteFile(dirFile, []byte(`{"type":"span","traceId":"trace-a","name":"chat gpt-5","attributes":{"gen_ai.operation.name":"chat","gen_ai.response.model":"gpt-5","gen_ai.conversation.id":"conv-a","gen_ai.usage.output_tokens":7}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(dir, "export.jsonl")
	if err := os.WriteFile(envFile, []byte(`{"type":"span","traceId":"trace-b","name":"chat gpt-5","attributes":{"gen_ai.operation.name":"chat","gen_ai.response.model":"gpt-5","gen_ai.conversation.id":"conv-b","gen_ai.usage.input_tokens":9}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(fileExporterPathEnv, envFile)

	events, err := (Source{}).Collect(context.Background(), config.Config{Paths: config.Paths{Copilot: []string{otel}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want 2", events)
	}
	bySession := map[string]int64{}
	for _, event := range events {
		bySession[event.SessionID] = event.Input + event.Output
	}
	if bySession["conv-a"] != 7 || bySession["conv-b"] != 9 {
		t.Fatalf("unexpected events by session: %+v", bySession)
	}
}
