package gemini

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/scan"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type Source struct{}

func (Source) Name() string { return "gemini" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	return scan.Parallel(ctx, cfg.Paths.Gemini,
		func(path string) bool {
			return strings.HasSuffix(path, ".json") && strings.Contains(filepath.Base(path), "session-")
		},
		ParseFile)
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
	body, err := os.ReadFile(path)
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
		events = append(events, usage.Event{
			Source:    "gemini",
			SessionID: session.SessionID,
			Project:   project,
			Model:     msg.Model,
			Input:     msg.Tokens.Input,
			Output:    msg.Tokens.Output + msg.Tokens.Tool,
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
