package kilo

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/usage"
	_ "modernc.org/sqlite"
)

const (
	dataDirEnv = "KILO_DATA_DIR"
	dbFileName = "kilo.db"
)

type Source struct{}

func (Source) Name() string { return "kilo" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	roots := append([]string(nil), cfg.Paths.Kilo...)
	if raw := os.Getenv(dataDirEnv); raw != "" {
		roots = splitEnvPaths(raw)
	}
	seen := map[string]bool{}
	var out []usage.Event
	for _, root := range roots {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		events, err := readDatabase(databasePath(root))
		if err != nil {
			continue
		}
		for _, event := range events {
			if seen[event.SessionID] {
				continue
			}
			seen[event.SessionID] = true
			out = append(out, event)
		}
	}
	return out, ctx.Err()
}

func databasePath(root string) string {
	if filepath.Base(root) == dbFileName {
		return root
	}
	return filepath.Join(root, dbFileName)
}

func splitEnvPaths(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func readDatabase(path string) ([]usage.Event, error) {
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return nil, nil
	}
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query("SELECT id, session_id, data FROM message")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []usage.Event
	for rows.Next() {
		var rowID, sessionID, data string
		if err := rows.Scan(&rowID, &sessionID, &data); err != nil {
			continue
		}
		if event, ok := parseMessage([]byte(data), rowID, sessionID); ok {
			out = append(out, event)
		}
	}
	return out, rows.Err()
}

type message struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	Model     string `json:"modelID"`
	Provider  string `json:"providerID"`
	Time      struct {
		Created int64 `json:"created"`
	} `json:"time"`
	Tokens struct {
		Input     int64 `json:"input"`
		Output    int64 `json:"output"`
		Reasoning int64 `json:"reasoning"`
		Total     int64 `json:"total"`
		Cache     struct {
			Read  int64 `json:"read"`
			Write int64 `json:"write"`
		} `json:"cache"`
	} `json:"tokens"`
}

func parseMessage(data []byte, rowID, rowSessionID string) (usage.Event, bool) {
	var msg message
	if err := json.Unmarshal(data, &msg); err != nil {
		return usage.Event{}, false
	}
	if msg.Role != "assistant" || strings.TrimSpace(msg.Model) == "" {
		return usage.Event{}, false
	}
	ts := timestampFromNumber(msg.Time.Created)
	if ts.IsZero() {
		return usage.Event{}, false
	}
	sessionID := strings.TrimSpace(msg.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(rowSessionID)
	}
	if msg.ID != "" {
		sessionID += "/" + msg.ID
	} else if rowID != "" {
		sessionID += "/" + rowID
	}
	input := nonNegative(msg.Tokens.Input)
	reasoning := nonNegative(msg.Tokens.Reasoning)
	output := nonNegative(msg.Tokens.Output) + reasoning
	cacheRead := nonNegative(msg.Tokens.Cache.Read)
	cacheCreate := nonNegative(msg.Tokens.Cache.Write)
	total := nonNegative(msg.Tokens.Total)
	if total > input+output+cacheRead+cacheCreate {
		output += total - (input + output + cacheRead + cacheCreate)
	}
	if input+output+cacheRead+cacheCreate == 0 {
		return usage.Event{}, false
	}
	return usage.Event{
		Source:      "kilo",
		SessionID:   sessionID,
		Project:     "kilo",
		Model:       msg.Model,
		Input:       input,
		Output:      output,
		CacheRead:   cacheRead,
		CacheCreate: cacheCreate,
		Reasoning:   reasoning,
		Timestamp:   ts,
	}, true
}

func timestampFromNumber(raw int64) time.Time {
	if raw <= 0 {
		return time.Time{}
	}
	if raw < 1_000_000_000_000 {
		raw *= 1000
	}
	return time.UnixMilli(raw).UTC()
}

func nonNegative(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
