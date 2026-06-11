package gemini

import (
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

type Source struct{}

func (Source) Name() string { return "gemini" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	return sourcepkg.CollectFiles(ctx, cfg, Source{})
}

func (Source) Roots(cfg config.Config) []string {
	return cfg.Paths.Gemini
}

func (Source) Match(path string) bool {
	return strings.HasSuffix(path, ".json") && strings.Contains(filepath.Base(path), "session-")
}

func (Source) Parse(path string, r io.Reader) ([]usage.Event, error) {
	return Parse(r, path)
}

type sessionFile struct {
	SessionID   string    `json:"sessionId"`
	StartTime   time.Time `json:"startTime"`
	LastUpdated time.Time `json:"lastUpdated"`
	Messages    []message `json:"messages"`
}

type message struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Model     string    `json:"model"`
	Tokens    struct {
		Input    int64 `json:"input"`
		Output   int64 `json:"output"`
		Cached   int64 `json:"cached"`
		Thoughts int64 `json:"thoughts"`
		Tool     int64 `json:"tool"`
		Total    int64 `json:"total"`
	} `json:"tokens"`
}

func ParseFile(path string) ([]usage.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f, path)
}

func Parse(r io.Reader, path string) ([]usage.Event, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var session sessionFile
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, err
	}

	project := projectName(path)
	events := make([]usage.Event, 0, len(session.Messages))
	for _, msg := range session.Messages {
		if msg.Model == "" && msg.Tokens.Total == 0 {
			continue
		}
		if msg.Tokens.Input+msg.Tokens.Output+msg.Tokens.Cached+msg.Tokens.Thoughts+msg.Tokens.Tool == 0 {
			continue
		}
		ts := msg.Timestamp
		if ts.IsZero() {
			ts = session.LastUpdated
		}
		if ts.IsZero() {
			ts = session.StartTime
		}
		// Gemini's prompt count (input) includes cached tokens; split them so
		// the disjoint components price correctly. Thoughts bill at the output
		// rate, so fold them into Output (and surface as the Reasoning subset).
		input := msg.Tokens.Input - msg.Tokens.Cached
		if input < 0 {
			input = 0
		}
		events = append(events, usage.Event{
			Source:    "gemini",
			SessionID: session.SessionID,
			Project:   project,
			Model:     msg.Model,
			Input:     input,
			Output:    msg.Tokens.Output + msg.Tokens.Thoughts + msg.Tokens.Tool,
			CacheRead: msg.Tokens.Cached,
			Reasoning: msg.Tokens.Thoughts,
			Timestamp: ts,
		})
	}
	return events, nil
}

func projectName(path string) string {
	dir := filepath.Dir(filepath.Dir(path))
	name := filepath.Base(dir)
	if name == "." || name == string(filepath.Separator) {
		return "unknown"
	}
	return name
}
