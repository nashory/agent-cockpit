package qwen

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	sourcepkg "github.com/nashory/agent-cockpit/internal/source"
	"github.com/nashory/agent-cockpit/internal/usage"
)

const (
	dataDirEnv   = "QWEN_DATA_DIR"
	defaultModel = "unknown"
)

type Source struct{}

func (Source) Name() string { return "qwen" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	return sourcepkg.CollectFiles(ctx, cfg, Source{})
}

func (Source) Roots(cfg config.Config) []string {
	if raw := os.Getenv(dataDirEnv); raw != "" {
		return splitEnvPaths(raw)
	}
	return cfg.Paths.Qwen
}

func (Source) Match(path string) bool {
	if filepath.Ext(path) != ".jsonl" {
		return false
	}
	parts := strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' })
	for i, part := range parts {
		if part == "projects" {
			return i+3 < len(parts) && parts[i+2] == "chats"
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

type qwenLine struct {
	Type          string        `json:"type"`
	Model         string        `json:"model"`
	Timestamp     string        `json:"timestamp"`
	SessionID     string        `json:"sessionId"`
	UsageMetadata usageMetadata `json:"usageMetadata"`
}

type usageMetadata struct {
	PromptTokenCount        int64 `json:"promptTokenCount"`
	CandidatesTokenCount    int64 `json:"candidatesTokenCount"`
	CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
	ThoughtsTokenCount      int64 `json:"thoughtsTokenCount"`
	TotalTokenCount         int64 `json:"totalTokenCount"`
}

func Parse(r io.Reader, path string) ([]usage.Event, error) {
	fallback := fileModifiedTimestamp(path)
	project := projectFromPath(path)
	sessionFallback := sessionIDFromPath(project, path)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	var events []usage.Event
	for scanner.Scan() {
		line := scanner.Bytes()
		if !strings.Contains(string(line), `"usageMetadata"`) {
			continue
		}
		var item qwenLine
		if err := json.Unmarshal(line, &item); err != nil {
			continue
		}
		if item.Type != "assistant" {
			continue
		}
		tokens := item.UsageMetadata
		input := nonNegative(tokens.PromptTokenCount)
		output := nonNegative(tokens.CandidatesTokenCount) + nonNegative(tokens.ThoughtsTokenCount)
		cacheRead := nonNegative(tokens.CachedContentTokenCount)
		total := nonNegative(tokens.TotalTokenCount)
		if total > input+output+cacheRead {
			output += total - (input + output + cacheRead)
		}
		if input+output+cacheRead == 0 {
			continue
		}
		ts := timestampFromString(item.Timestamp)
		if ts.IsZero() {
			ts = fallback
		}
		model := strings.TrimSpace(item.Model)
		if model == "" {
			model = defaultModel
		}
		sessionID := strings.TrimSpace(item.SessionID)
		if sessionID == "" {
			sessionID = sessionFallback
		}
		events = append(events, usage.Event{
			Source:    "qwen",
			SessionID: sessionID,
			Project:   project,
			Model:     model,
			Input:     input,
			Output:    output,
			CacheRead: cacheRead,
			Reasoning: nonNegative(tokens.ThoughtsTokenCount),
			Timestamp: ts,
		})
	}
	return events, scanner.Err()
}

func projectFromPath(path string) string {
	parts := strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' })
	for i, part := range parts {
		if part == "projects" && i+1 < len(parts) {
			project := strings.TrimSpace(parts[i+1])
			if project != "" {
				return project
			}
		}
	}
	return "qwen"
}

func sessionIDFromPath(project, path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if strings.TrimSpace(stem) == "" {
		stem = "unknown"
	}
	if strings.TrimSpace(project) == "" || project == "qwen" {
		return stem
	}
	return project + "-" + stem
}

func timestampFromString(text string) time.Time {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return time.Time{}
	}
	return ts.UTC()
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
