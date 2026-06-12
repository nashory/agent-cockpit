package goose

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/usage"
	_ "modernc.org/sqlite"
)

const (
	pathRootEnv = "GOOSE_PATH_ROOT"
	dbFileName  = "sessions.db"
)

type Source struct{}

func (Source) Name() string { return "goose" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	var paths []string
	if raw := strings.TrimSpace(os.Getenv(pathRootEnv)); raw != "" {
		paths = []string{filepath.Join(raw, "data", "sessions", dbFileName)}
	} else {
		paths = dbPaths(cfg.Paths.Goose)
	}
	seen := map[string]bool{}
	var out []usage.Event
	for _, path := range paths {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		events, err := readDatabase(path)
		if err != nil {
			continue
		}
		for _, event := range events {
			key := path + "\x00" + event.SessionID
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, event)
		}
	}
	return out, ctx.Err()
}

func dbPaths(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		if filepath.Base(root) == dbFileName {
			out = append(out, root)
		} else {
			out = append(out, filepath.Join(root, dbFileName))
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
	rows, err := db.Query(`
SELECT
  id,
  model_config_json,
  provider_name,
  created_at,
  total_tokens,
  input_tokens,
  output_tokens,
  accumulated_total_tokens,
  accumulated_input_tokens,
  accumulated_output_tokens
FROM sessions
WHERE model_config_json IS NOT NULL AND TRIM(model_config_json) != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []usage.Event
	for rows.Next() {
		var row gooseRow
		if err := rows.Scan(&row.id, &row.modelConfig, &row.provider, &row.createdAt, &row.total, &row.input, &row.output, &row.accumTotal, &row.accumInput, &row.accumOutput); err != nil {
			continue
		}
		if event, ok := row.toEvent(); ok {
			out = append(out, event)
		}
	}
	return out, rows.Err()
}

type gooseRow struct {
	id          string
	modelConfig string
	provider    sql.NullString
	createdAt   anyTime
	total       sql.NullInt64
	input       sql.NullInt64
	output      sql.NullInt64
	accumTotal  sql.NullInt64
	accumInput  sql.NullInt64
	accumOutput sql.NullInt64
}

func (r gooseRow) toEvent() (usage.Event, bool) {
	model := parseModel(r.modelConfig)
	if model == "" {
		return usage.Event{}, false
	}
	ts := parseTimestamp(r.createdAt.text)
	if ts.IsZero() {
		return usage.Event{}, false
	}
	input := nullInt(r.accumInput)
	if input == 0 {
		input = nullInt(r.input)
	}
	output := nullInt(r.accumOutput)
	if output == 0 {
		output = nullInt(r.output)
	}
	total := nullInt(r.accumTotal)
	if total == 0 {
		total = nullInt(r.total)
	}
	reasoning := int64(0)
	if total > input+output {
		reasoning = total - (input + output)
		output += reasoning
	}
	if input+output == 0 {
		return usage.Event{}, false
	}
	return usage.Event{
		Source:    "goose",
		SessionID: r.id,
		Project:   "goose",
		Model:     model,
		Input:     input,
		Output:    output,
		Reasoning: reasoning,
		Timestamp: ts,
	}, true
}

type anyTime struct {
	text string
}

func (t *anyTime) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		t.text = ""
	case string:
		t.text = v
	case []byte:
		t.text = string(v)
	case int64:
		t.text = strconv.FormatInt(v, 10)
	default:
		t.text = ""
	}
	return nil
}

func parseModel(raw string) string {
	var cfg struct {
		ModelName string `json:"model_name"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.ModelName)
}

func parseTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if number, ok := parseInt(raw); ok {
		if number < 1_000_000_000_000 {
			number *= 1000
		}
		return time.UnixMilli(number).UTC()
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts.UTC()
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if ts, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return ts.UTC()
		}
	}
	return time.Time{}
}

func parseInt(raw string) (int64, bool) {
	var out int64
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		out = out*10 + int64(ch-'0')
	}
	return out, out > 0
}

func nullInt(v sql.NullInt64) int64 {
	if v.Valid && v.Int64 > 0 {
		return v.Int64
	}
	return 0
}
