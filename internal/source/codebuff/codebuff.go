package codebuff

import (
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
	dataDirEnv   = "CODEBUFF_DATA_DIR"
	defaultModel = "codebuff-unknown"
)

type Source struct{}

func (Source) Name() string { return "codebuff" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	return sourcepkg.CollectFiles(ctx, cfg, Source{})
}

func (Source) Roots(cfg config.Config) []string {
	if raw := os.Getenv(dataDirEnv); raw != "" {
		return projectRoots(splitEnvPaths(raw))
	}
	return projectRoots(cfg.Paths.Codebuff)
}

func (Source) Match(path string) bool {
	if filepath.Base(path) != "chat-messages.json" {
		return false
	}
	parts := strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' })
	for i, part := range parts {
		if part == "projects" {
			return i+4 < len(parts) && parts[i+2] == "chats"
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

func projectRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		if filepath.Base(root) == "projects" {
			out = append(out, root)
		} else {
			out = append(out, filepath.Join(root, "projects"))
		}
	}
	return out
}

type message map[string]any

type assistantUsage struct {
	model       string
	input       int64
	output      int64
	cacheCreate int64
	cacheRead   int64
	extraTotal  int64
}

func Parse(r io.Reader, path string) ([]usage.Event, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var messages []message
	if err := json.Unmarshal(body, &messages); err != nil {
		return nil, nil
	}
	ctx := deriveContext(path)
	chatTimestamp := parseChatIDTimestamp(ctx.chatID)
	fileTimestamp := fileModifiedTimestamp(path)
	events := make([]usage.Event, 0, len(messages))
	for ordinal, msg := range messages {
		if !isAssistantMessage(msg) {
			continue
		}
		tokens := extractAssistantUsage(msg)
		if !tokens.hasSignal() {
			continue
		}
		ts := timestampValue(msg["timestamp"])
		if ts.IsZero() {
			ts = timestampValue(msg["createdAt"])
		}
		if ts.IsZero() {
			if metadata, ok := objectField(msg, "metadata"); ok {
				ts = timestampValue(metadata["timestamp"])
			}
		}
		if ts.IsZero() {
			ts = chatTimestamp
		}
		if ts.IsZero() {
			ts = fileTimestamp
		}
		model := tokens.model
		if strings.TrimSpace(model) == "" {
			model = defaultModel
		}
		sessionID := ctx.sessionID
		if id := stringField(msg, "id"); id != "" {
			sessionID += "/" + id
		} else {
			sessionID += "/" + strconv.Itoa(ordinal)
		}
		events = append(events, usage.Event{
			Source:      "codebuff",
			SessionID:   sessionID,
			Project:     ctx.project,
			Model:       model,
			Input:       tokens.input,
			Output:      tokens.output + tokens.extraTotal,
			CacheRead:   tokens.cacheRead,
			CacheCreate: tokens.cacheCreate,
			Timestamp:   ts,
		})
	}
	return events, nil
}

type contextInfo struct {
	channel   string
	project   string
	chatID    string
	sessionID string
}

func deriveContext(path string) contextInfo {
	chatID := filepath.Base(filepath.Dir(path))
	parts := strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' })
	channel := "manicode"
	project := "unknown"
	for i, part := range parts {
		if part == "projects" {
			if i > 0 {
				channel = parts[i-1]
			}
			if i+1 < len(parts) {
				project = parts[i+1]
			}
			break
		}
	}
	session := channel + "/" + project + "/" + chatID
	return contextInfo{channel: channel, project: project, chatID: chatID, sessionID: session}
}

func isAssistantMessage(msg message) bool {
	role := stringField(msg, "variant")
	if role == "" {
		role = stringField(msg, "role")
	}
	return role == "ai" || role == "agent" || role == "assistant"
}

func extractAssistantUsage(msg message) assistantUsage {
	var out assistantUsage
	if metadata, ok := objectField(msg, "metadata"); ok {
		out.model = stringField(metadata, "model")
		out.mergeFallback(parseUsageObject(metadata["usage"]))
		if codebuff, ok := objectField(metadata, "codebuff"); ok {
			out.mergeFallback(parseUsageObject(codebuff["usage"]))
		}
		if runStateUsage, ok := extractUsageFromRunState(metadata); ok {
			out.mergeFallback(runStateUsage)
		}
	}
	return out
}

func extractUsageFromRunState(metadata message) (assistantUsage, bool) {
	history, ok := arrayAt(metadata, "runState", "sessionState", "mainAgentState", "messageHistory")
	if !ok {
		return assistantUsage{}, false
	}
	var out assistantUsage
	found := false
	for i := len(history) - 1; i >= 0; i-- {
		entry, ok := history[i].(map[string]any)
		if !ok || stringField(entry, "role") != "assistant" {
			continue
		}
		providerOptions, ok := objectField(entry, "providerOptions")
		if !ok {
			continue
		}
		var item assistantUsage
		item.mergeFallback(parseUsageObject(providerOptions["usage"]))
		if codebuff, ok := objectField(providerOptions, "codebuff"); ok {
			item.mergeFallback(parseUsageObject(codebuff["usage"]))
			if model := stringField(codebuff, "model"); model != "" && item.model == "" {
				item.model = model
			}
		}
		if item.hasSignal() || item.model != "" {
			found = true
		}
		out.mergeFallback(item)
	}
	return out, found
}

func parseUsageObject(value any) assistantUsage {
	record, ok := value.(map[string]any)
	if !ok {
		return assistantUsage{}
	}
	out := assistantUsage{
		input:       pickNumber(record, "inputTokens", "input_tokens", "promptTokens", "prompt_tokens"),
		output:      pickNumber(record, "outputTokens", "output_tokens", "completionTokens", "completion_tokens"),
		cacheRead:   pickNumber(record, "cacheReadInputTokens", "cache_read_input_tokens"),
		cacheCreate: pickNumber(record, "cacheCreationInputTokens", "cache_creation_input_tokens", "cacheCreationTokens", "cache_creation_tokens", "cachedTokensCreated", "cached_tokens_created"),
		model:       stringField(record, "model"),
	}
	out.cacheRead = maxInt64(out.cacheRead, pickNestedNumber(record, "promptTokensDetails", "cachedTokens"))
	out.cacheRead = maxInt64(out.cacheRead, pickNestedNumber(record, "prompt_tokens_details", "cached_tokens"))
	total := pickNumber(record, "totalTokens", "total_tokens", "total")
	if total > out.input+out.output+out.cacheRead+out.cacheCreate {
		out.extraTotal = total - (out.input + out.output + out.cacheRead + out.cacheCreate)
	}
	return out
}

func (u *assistantUsage) mergeFallback(fallback assistantUsage) {
	if u.input == 0 {
		u.input = fallback.input
	}
	if u.output == 0 {
		u.output = fallback.output
	}
	if u.cacheCreate == 0 {
		u.cacheCreate = fallback.cacheCreate
	}
	if u.cacheRead == 0 {
		u.cacheRead = fallback.cacheRead
	}
	if u.extraTotal == 0 {
		u.extraTotal = fallback.extraTotal
	}
	if u.model == "" {
		u.model = fallback.model
	}
}

func (u assistantUsage) hasSignal() bool {
	return u.input+u.output+u.cacheCreate+u.cacheRead+u.extraTotal > 0
}

func objectField(record map[string]any, key string) (map[string]any, bool) {
	nested, ok := record[key].(map[string]any)
	return nested, ok
}

func stringField(record map[string]any, key string) string {
	value, _ := record[key].(string)
	return strings.TrimSpace(value)
}

func pickNumber(record map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value := numberValue(record[key]); value > 0 {
			return value
		}
	}
	return 0
}

func pickNestedNumber(record map[string]any, key string, keys ...string) int64 {
	nested, ok := objectField(record, key)
	if !ok {
		return 0
	}
	return pickNumber(nested, keys...)
}

func numberValue(value any) int64 {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return 0
		}
		return int64(v)
	case int64:
		if v > 0 {
			return v
		}
	case json.Number:
		n, err := v.Int64()
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func arrayAt(record map[string]any, keys ...string) ([]any, bool) {
	var current any = record
	for _, key := range keys {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current = obj[key]
	}
	array, ok := current.([]any)
	return array, ok
}

func timestampValue(value any) time.Time {
	switch v := value.(type) {
	case string:
		return parseTimestampString(v)
	case float64:
		return timestampFromNumber(int64(v))
	case int64:
		return timestampFromNumber(v)
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return time.Time{}
		}
		return timestampFromNumber(n)
	default:
		return time.Time{}
	}
}

func timestampFromNumber(raw int64) time.Time {
	if raw <= 0 {
		return time.Time{}
	}
	if raw < 10_000_000_000 {
		raw *= 1000
	}
	return time.UnixMilli(raw).UTC()
}

func parseTimestampString(text string) time.Time {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339Nano, text); err == nil {
		return ts.UTC()
	}
	if ts, err := time.Parse("2006-01-02T15:04:05", text); err == nil {
		return ts.UTC()
	}
	return time.Time{}
}

func parseChatIDTimestamp(chatID string) time.Time {
	date, clock, ok := strings.Cut(chatID, "T")
	if !ok {
		return time.Time{}
	}
	for i := 0; i < 2; i++ {
		if idx := strings.IndexByte(clock, '-'); idx >= 0 {
			clock = clock[:idx] + ":" + clock[idx+1:]
		}
	}
	return parseTimestampString(date + "T" + clock)
}

func fileModifiedTimestamp(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime().UTC()
}

func maxInt64(a, b int64) int64 {
	if b > a {
		return b
	}
	return a
}
