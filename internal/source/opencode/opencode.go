package opencode

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/usage"
	_ "modernc.org/sqlite"
)

type Source struct{}

func (Source) Name() string { return "opencode" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	roots := append([]string(nil), cfg.Paths.OpenCode...)
	if raw := os.Getenv("OPENCODE_DATA_DIR"); raw != "" {
		roots = splitEnvPaths(raw)
	}

	var all []usage.Event
	seen := map[string]bool{}
	for _, root := range roots {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		events, err := collectRoot(ctx, root)
		if err != nil {
			continue
		}
		for _, e := range events {
			id := e.SessionID + "\x00" + e.Model + "\x00" + e.Timestamp.Format(time.RFC3339Nano)
			if seen[id] {
				continue
			}
			seen[id] = true
			all = append(all, e)
		}
	}
	return all, nil
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

func collectRoot(ctx context.Context, root string) ([]usage.Event, error) {
	if root == "" {
		return nil, nil
	}
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	seenMessages := map[string]bool{}
	var events []usage.Event
	if db := databasePath(root); db != "" {
		rows, err := readDatabase(db)
		if err == nil {
			for _, row := range rows {
				if row.messageID != "" {
					seenMessages[row.messageID] = true
				}
				events = append(events, row.event)
			}
		}
	}

	messageDir := filepath.Join(root, "storage", "message")
	_ = filepath.WalkDir(messageDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		event, messageID, ok := readMessageFile(path)
		if !ok || (messageID != "" && seenMessages[messageID]) {
			return nil
		}
		events = append(events, event)
		return nil
	})

	sort.Slice(events, func(i, j int) bool { return events[i].Timestamp.Before(events[j].Timestamp) })
	return events, ctx.Err()
}

func databasePath(root string) string {
	defaultPath := filepath.Join(root, "opencode.db")
	if isFile(defaultPath) {
		return defaultPath
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	var candidates []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && isChannelDBName(name) {
			candidates = append(candidates, filepath.Join(root, name))
		}
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

func isChannelDBName(name string) bool {
	if !strings.HasPrefix(name, "opencode-") || !strings.HasSuffix(name, ".db") {
		return false
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(name, "opencode-"), ".db")
	if mid == "" {
		return false
	}
	for _, ch := range mid {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}

type dbEvent struct {
	event     usage.Event
	messageID string
}

func readDatabase(path string) ([]dbEvent, error) {
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, session_id, data FROM message")
	if err == nil {
		defer rows.Close()
		var out []dbEvent
		for rows.Next() {
			var id, sessionID, data string
			if err := rows.Scan(&id, &sessionID, &data); err != nil {
				continue
			}
			event, messageID, ok := parseMessage(strings.NewReader(data), id, sessionID)
			if ok {
				out = append(out, dbEvent{event: event, messageID: messageID})
			}
		}
		return out, rows.Err()
	}

	return readLegacySessions(db)
}

func readLegacySessions(db *sql.DB) ([]dbEvent, error) {
	rows, err := db.Query(`
SELECT
  s.id,
  COALESCE(
    (SELECT m.model FROM messages m WHERE m.session_id = s.id AND m.model IS NOT NULL AND m.model != '' ORDER BY m.updated_at DESC LIMIT 1),
    'opencode'
  ) AS model,
  s.prompt_tokens,
  s.completion_tokens,
  s.updated_at,
  s.created_at
FROM sessions s
WHERE s.prompt_tokens + s.completion_tokens > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []dbEvent
	for rows.Next() {
		var sessionID, model string
		var input, output, updated, created int64
		if err := rows.Scan(&sessionID, &model, &input, &output, &updated, &created); err != nil {
			continue
		}
		ts := unixTime(updated)
		if ts.IsZero() {
			ts = unixTime(created)
		}
		out = append(out, dbEvent{event: usage.Event{
			Source:    "opencode",
			SessionID: sessionID,
			Project:   "opencode",
			Model:     model,
			Input:     input,
			Output:    output,
			Timestamp: ts,
		}})
	}
	return out, rows.Err()
}

func readMessageFile(path string) (usage.Event, string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return usage.Event{}, "", false
	}
	defer f.Close()
	return parseMessage(f, "", "")
}

type message struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	Provider  string `json:"providerID"`
	Model     string `json:"modelID"`
	Time      struct {
		Created int64 `json:"created"`
	} `json:"time"`
	Tokens struct {
		Input  int64 `json:"input"`
		Output int64 `json:"output"`
		Total  int64 `json:"total"`
		Cache  struct {
			Read  int64 `json:"read"`
			Write int64 `json:"write"`
		} `json:"cache"`
	} `json:"tokens"`
}

func parseMessage(r io.Reader, fallbackID, fallbackSessionID string) (usage.Event, string, bool) {
	var msg message
	if err := json.NewDecoder(r).Decode(&msg); err != nil {
		return usage.Event{}, "", false
	}
	if msg.ID == "" {
		msg.ID = fallbackID
	}
	if msg.SessionID == "" {
		msg.SessionID = fallbackSessionID
	}
	if msg.Model == "" {
		return usage.Event{}, "", false
	}

	input := nonNegative(msg.Tokens.Input)
	output := nonNegative(msg.Tokens.Output)
	cacheRead := nonNegative(msg.Tokens.Cache.Read)
	cacheCreate := nonNegative(msg.Tokens.Cache.Write)
	total := nonNegative(msg.Tokens.Total)
	known := input + output + cacheRead + cacheCreate
	if total > known {
		output += total - known
	}
	if input+output+cacheRead+cacheCreate == 0 {
		return usage.Event{}, "", false
	}

	return usage.Event{
		Source:      "opencode",
		SessionID:   msg.SessionID,
		Project:     "opencode",
		Model:       normalizeModel(msg.Model),
		Input:       input,
		Output:      output,
		CacheRead:   cacheRead,
		CacheCreate: cacheCreate,
		Timestamp:   unixTime(msg.Time.Created),
	}, msg.ID, true
}

func nonNegative(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func unixTime(v int64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	if v > 1_000_000_000_000 {
		return time.UnixMilli(v).UTC()
	}
	return time.Unix(v, 0).UTC()
}

func normalizeModel(model string) string {
	switch model {
	case "gemini-3-pro-high":
		return "gemini-3-pro-preview"
	case "k2p6":
		return "kimi-k2.6"
	default:
		return model
	}
}
