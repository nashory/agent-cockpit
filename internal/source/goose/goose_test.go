package goose

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	_ "modernc.org/sqlite"
)

func TestReadDatabaseLoadsAccumulatedTokens(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, dbFileName)
	createGooseDB(t, dbPath)
	insertSession(t, dbPath, "session-a", `{"model_name":"claude-sonnet-4-20250514"}`, "anthropic", "2026-05-01 01:02:03", 180, 100, 50)

	events, err := readDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	event := events[0]
	if event.Source != "goose" || event.SessionID != "session-a" || event.Project != "goose" || event.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("unexpected identity: %+v", event)
	}
	if event.Input != 100 || event.Output != 80 || event.Reasoning != 30 {
		t.Fatalf("unexpected tokens: %+v", event)
	}
	want := time.Date(2026, 5, 1, 1, 2, 3, 0, time.UTC)
	if !event.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", event.Timestamp, want)
	}
}

func TestCollectUsesGoosePathRoot(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data", "sessions", dbFileName)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	createGooseDB(t, dbPath)
	insertSession(t, dbPath, "session-a", `{"model_name":"gpt-5"}`, "openai", "1767312000000", 0, 10, 5)
	t.Setenv(pathRootEnv, dir)

	events, err := (Source{}).Collect(context.Background(), config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Input != 10 || events[0].Output != 5 {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestReadDatabaseSkipsRowsWithoutUsefulFields(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, dbFileName)
	createGooseDB(t, dbPath)
	insertSession(t, dbPath, "missing-model", `{}`, "", "2026-05-01", 1, 1, 0)
	insertSession(t, dbPath, "missing-time", `{"model_name":"gpt-5"}`, "", "", 1, 1, 0)
	insertSession(t, dbPath, "zero", `{"model_name":"gpt-5"}`, "", "2026-05-01", 0, 0, 0)

	events, err := readDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("events should be skipped: %+v", events)
	}
}

func createGooseDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  model_config_json TEXT,
  provider_name TEXT,
  created_at TEXT,
  total_tokens INTEGER,
  input_tokens INTEGER,
  output_tokens INTEGER,
  accumulated_total_tokens INTEGER,
  accumulated_input_tokens INTEGER,
  accumulated_output_tokens INTEGER
)`)
	if err != nil {
		t.Fatal(err)
	}
}

func insertSession(t *testing.T, path, id, modelConfig, provider, createdAt string, total, input, output int64) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stmt, err := db.Prepare(`
INSERT INTO sessions (
  id,
  model_config_json,
  provider_name,
  created_at,
  accumulated_total_tokens,
  accumulated_input_tokens,
  accumulated_output_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(id, modelConfig, provider, createdAt, total, input, output); err != nil {
		t.Fatal(err)
	}
}
