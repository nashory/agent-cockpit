package kilo

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	_ "modernc.org/sqlite"
)

func TestParseMessage(t *testing.T) {
	event, ok := parseMessage([]byte(`{"id":"msg-1","role":"assistant","providerID":"anthropic","modelID":"claude-sonnet-4-20250514","time":{"created":1767312000000},"tokens":{"input":100,"output":50,"reasoning":5,"cache":{"read":10,"write":20}}}`), "row-1", "session-a")
	if !ok {
		t.Fatal("expected event")
	}
	if event.Source != "kilo" || event.SessionID != "session-a/msg-1" || event.Project != "kilo" || event.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("unexpected identity: %+v", event)
	}
	if event.Input != 100 || event.Output != 55 || event.CacheRead != 10 || event.CacheCreate != 20 || event.Reasoning != 5 {
		t.Fatalf("unexpected tokens: %+v", event)
	}
	want := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if !event.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", event.Timestamp, want)
	}
}

func TestParseMessageUsesTotalFallback(t *testing.T) {
	event, ok := parseMessage([]byte(`{"role":"assistant","modelID":"gpt-5","time":{"created":1767312000},"tokens":{"total":123}}`), "row-1", "session-a")
	if !ok {
		t.Fatal("expected event")
	}
	if event.Output != 123 || event.TotalTokens() != 123 {
		t.Fatalf("total fallback not mapped: %+v", event)
	}
}

func TestCollectLoadsKiloDatabaseAndDedupes(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	createKiloDB(t, filepath.Join(first, dbFileName), "row-1", "session-a", `{"id":"msg-1","role":"assistant","providerID":"openai","modelID":"gpt-5","time":{"created":1767312000000},"tokens":{"input":10,"output":1}}`)
	createKiloDB(t, filepath.Join(second, dbFileName), "row-2", "session-a", `{"id":"msg-1","role":"assistant","providerID":"openai","modelID":"gpt-5","time":{"created":1767312000000},"tokens":{"input":20,"output":1}}`)
	t.Setenv(dataDirEnv, first+","+second)

	events, err := (Source{}).Collect(context.Background(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1 deduped event", events)
	}
	if events[0].Input != 10 {
		t.Fatalf("first event should win dedupe: %+v", events[0])
	}
}

func TestCollectAcceptsKiloDatabaseFilePath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, dbFileName)
	createKiloDB(t, dbPath, "row-1", "session-a", `{"id":"msg-1","role":"assistant","providerID":"openai","modelID":"gpt-5","time":{"created":1767312000000},"tokens":{"input":10,"output":1}}`)
	t.Setenv(dataDirEnv, dbPath)

	events, err := (Source{}).Collect(context.Background(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want database file path to be accepted", events)
	}
}

func TestParseSkipsNonAssistantAndMissingTimestamp(t *testing.T) {
	for _, body := range []string{
		`{"role":"user","modelID":"gpt-5","time":{"created":1767312000},"tokens":{"input":1}}`,
		`{"role":"assistant","modelID":"gpt-5","tokens":{"input":1}}`,
		`{"role":"assistant","modelID":"gpt-5","time":{"created":1767312000},"tokens":{}}`,
	} {
		if event, ok := parseMessage([]byte(body), "row-1", "session-a"); ok {
			t.Fatalf("message should be skipped: %+v", event)
		}
	}
}

func createKiloDB(t *testing.T, path, id, sessionID, data string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE message (id TEXT, session_id TEXT, data TEXT)"); err != nil {
		t.Fatal(err)
	}
	stmt, err := db.Prepare("INSERT INTO message (id, session_id, data) VALUES (?, ?, ?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(id, sessionID, data); err != nil {
		t.Fatal(err)
	}
}
