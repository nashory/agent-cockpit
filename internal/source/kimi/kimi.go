package kimi

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	sourcepkg "github.com/nashory/agent-cockpit/internal/source"
	"github.com/nashory/agent-cockpit/internal/usage"
)

const (
	dataDirEnv   = "KIMI_DATA_DIR"
	defaultModel = "kimi-for-coding"
)

type Source struct{}

func (Source) Name() string { return "kimi" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	events, err := sourcepkg.CollectFiles(ctx, cfg, Source{})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]usage.Event, 0, len(events))
	for _, event := range events {
		key := eventKey(event)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, event)
	}
	return out, nil
}

func (Source) Roots(cfg config.Config) []string {
	if raw := os.Getenv(dataDirEnv); raw != "" {
		return splitEnvPaths(raw)
	}
	return cfg.Paths.Kimi
}

func (Source) Match(path string) bool {
	if filepath.Base(path) != "wire.jsonl" {
		return false
	}
	parts := strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' })
	for i, part := range parts {
		if part == "sessions" {
			return len(parts)-i == 4
		}
	}
	return false
}

func (Source) Parse(path string, r io.Reader) ([]usage.Event, error) {
	return Parse(r, path)
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

type wireLine struct {
	Type      string      `json:"type"`
	Timestamp jsonNumber  `json:"timestamp"`
	Message   wireMessage `json:"message"`
}

type wireMessage struct {
	Type    string      `json:"type"`
	Payload wirePayload `json:"payload"`
}

type wirePayload struct {
	MessageID  string     `json:"message_id"`
	TokenUsage tokenUsage `json:"token_usage"`
}

type tokenUsage struct {
	InputOther       int64 `json:"input_other"`
	Output           int64 `json:"output"`
	InputCacheRead   int64 `json:"input_cache_read"`
	InputCacheCreate int64 `json:"input_cache_creation"`
	Total            int64 `json:"total"`
}

type jsonNumber struct {
	text string
}

func (n *jsonNumber) UnmarshalJSON(b []byte) error {
	n.text = strings.Trim(string(b), `"`)
	return nil
}

func Parse(r io.Reader, path string) ([]usage.Event, error) {
	model := modelFromConfig(path)
	sessionID := sessionIDFromPath(path)
	fallback := fileModifiedTimestamp(path)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	var events []usage.Event
	for scanner.Scan() {
		line := scanner.Bytes()
		if !strings.Contains(string(line), `"StatusUpdate"`) || !strings.Contains(string(line), `"token_usage"`) {
			continue
		}
		var item wireLine
		if err := json.Unmarshal(line, &item); err != nil {
			continue
		}
		if item.Type == "metadata" || item.Message.Type != "StatusUpdate" {
			continue
		}
		tokens := item.Message.Payload.TokenUsage
		input := nonNegative(tokens.InputOther)
		output := nonNegative(tokens.Output)
		cacheRead := nonNegative(tokens.InputCacheRead)
		cacheCreate := nonNegative(tokens.InputCacheCreate)
		total := nonNegative(tokens.Total)
		if total > input+output+cacheRead+cacheCreate {
			output += total - (input + output + cacheRead + cacheCreate)
		}
		if input+output+cacheRead+cacheCreate == 0 {
			continue
		}
		ts := timestampFromSeconds(item.Timestamp.text)
		if ts.IsZero() {
			ts = fallback
		}
		events = append(events, usage.Event{
			Source:      "kimi",
			SessionID:   sessionID,
			Project:     "kimi",
			Model:       model,
			Input:       input,
			Output:      output,
			CacheRead:   cacheRead,
			CacheCreate: cacheCreate,
			Timestamp:   ts,
		})
	}
	return events, scanner.Err()
}

func modelFromConfig(path string) string {
	root := kimiRootFromWirePath(path)
	if root == "" {
		return defaultModel
	}
	body, err := os.ReadFile(filepath.Join(root, "config.json"))
	if err != nil {
		return defaultModel
	}
	var cfg struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &cfg); err != nil {
		return defaultModel
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return defaultModel
	}
	return cfg.Model
}

func kimiRootFromWirePath(path string) string {
	dir := filepath.Dir(path)
	for i := 0; i < 2; i++ {
		dir = filepath.Dir(dir)
	}
	if filepath.Base(dir) != "sessions" {
		return ""
	}
	return filepath.Dir(dir)
}

func sessionIDFromPath(path string) string {
	session := filepath.Base(filepath.Dir(path))
	if strings.TrimSpace(session) == "" || session == "." || session == string(filepath.Separator) {
		return "unknown"
	}
	return session
}

func timestampFromSeconds(text string) time.Time {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return time.Time{}
	}
	millis := int64(seconds * 1000)
	return time.UnixMilli(millis).UTC()
}

func fileModifiedTimestamp(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime().UTC()
}

func nonNegative(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func eventKey(event usage.Event) string {
	return strings.Join([]string{
		event.SessionID,
		event.Model,
		event.Timestamp.Format(time.RFC3339Nano),
		strconv.FormatInt(event.Input, 10),
		strconv.FormatInt(event.Output, 10),
		strconv.FormatInt(event.CacheRead, 10),
		strconv.FormatInt(event.CacheCreate, 10),
	}, "\x00")
}
