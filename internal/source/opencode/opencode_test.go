package opencode

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	_ "modernc.org/sqlite"
)

func TestParseMessage(t *testing.T) {
	body := `{"id":"msg-1","sessionID":"session-a","providerID":"anthropic","modelID":"claude-sonnet-4.5","time":{"created":1767312000000},"tokens":{"input":100,"output":50,"cache":{"read":10,"write":20}}}`
	event, id, ok := parseMessage(stringsReader(body), "", "")
	if !ok {
		t.Fatal("expected event")
	}
	if id != "msg-1" || event.Source != "opencode" || event.SessionID != "session-a" || event.Model != "claude-sonnet-4.5" {
		t.Fatalf("unexpected event identity: id=%q event=%+v", id, event)
	}
	if event.Input != 100 || event.Output != 50 || event.CacheRead != 10 || event.CacheCreate != 20 {
		t.Fatalf("unexpected token mapping: %+v", event)
	}
	want := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if !event.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", event.Timestamp, want)
	}
}

func TestParseMessageUsesTotalFallback(t *testing.T) {
	event, _, ok := parseMessage(stringsReader(`{"sessionID":"s","modelID":"gpt-test","time":{"created":1},"tokens":{"total":123}}`), "m", "")
	if !ok {
		t.Fatal("expected event")
	}
	if event.Output != 123 || event.TotalTokens() != 123 {
		t.Fatalf("total fallback not mapped to output: %+v", event)
	}
}

func TestCollectLoadsDatabaseAndDedupesMessageJSON(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")
	createMessageDB(t, dbPath, "msg-1", "session-db", `{"modelID":"gpt-5-codex","time":{"created":1767312000000},"tokens":{"input":100,"output":50}}`)

	msgDir := filepath.Join(dir, "storage", "message")
	if err := os.MkdirAll(msgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(msgDir, "same.json"), []byte(`{"id":"msg-1","sessionID":"session-json","modelID":"gpt-5-codex","time":{"created":1767312001000},"tokens":{"input":999}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(msgDir, "new.json"), []byte(`{"id":"msg-2","sessionID":"session-json","modelID":"claude-sonnet-4-5","time":{"created":1767312002000},"tokens":{"input":10,"output":5}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := (Source{}).Collect(context.Background(), config.Config{Paths: config.Paths{OpenCode: []string{dir}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want 2 deduped events", events)
	}
	if events[0].SessionID != "session-db" || events[0].Input != 100 {
		t.Fatalf("database event missing or unsorted: %+v", events)
	}
	if events[1].SessionID != "session-json" || events[1].Input != 10 {
		t.Fatalf("json fallback event missing: %+v", events)
	}
}

func TestReadLegacySessionsDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE TABLE sessions (id TEXT, prompt_tokens INTEGER, completion_tokens INTEGER, updated_at INTEGER, created_at INTEGER);
CREATE TABLE messages (session_id TEXT, model TEXT, updated_at INTEGER);
INSERT INTO sessions VALUES ('session-1', 100, 50, 1767312000, 1767311000);
INSERT INTO messages VALUES ('session-1', 'claude-sonnet-4-5', 1767312000);`)
	if err != nil {
		t.Fatal(err)
	}

	events, err := readDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	event := events[0].event
	if event.SessionID != "session-1" || event.Model != "claude-sonnet-4-5" || event.Input != 100 || event.Output != 50 {
		t.Fatalf("unexpected legacy event: %+v", event)
	}
}

func createMessageDB(t *testing.T, path, id, sessionID, data string) {
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

func stringsReader(s string) *strings.Reader {
	return strings.NewReader(s)
}
