package codex

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
	"github.com/nashory/agent-cockpit/internal/scan"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type Source struct{}

func (Source) Name() string { return "codex" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	return scan.Parallel(ctx, cfg.Paths.Codex,
		func(path string) bool { return strings.HasSuffix(path, ".jsonl") },
		ParseFile)
}

type envelope struct {
	Timestamp time.Time       `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type metaPayload struct {
	ID    string `json:"id"`
	CWD   string `json:"cwd"`
	Model string `json:"model"`
}

type tokenPayload struct {
	Type string `json:"type"`
	Info struct {
		Last struct {
			Input     int64 `json:"input_tokens"`
			Cached    int64 `json:"cached_input_tokens"`
			Output    int64 `json:"output_tokens"`
			Reasoning int64 `json:"reasoning_output_tokens"`
		} `json:"last_token_usage"`
	} `json:"info"`
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
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	var events []usage.Event
	var sessionID, cwd, model string
	for scanner.Scan() {
		var env envelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			continue
		}
		switch env.Type {
		case "session_meta":
			var meta metaPayload
			if err := json.Unmarshal(env.Payload, &meta); err == nil {
				sessionID = meta.ID
				cwd = meta.CWD
				model = meta.Model
			}
		case "event_msg":
			var p tokenPayload
			if err := json.Unmarshal(env.Payload, &p); err != nil || p.Type != "token_count" {
				continue
			}
			if p.Info.Last.Input+p.Info.Last.Cached+p.Info.Last.Output+p.Info.Last.Reasoning == 0 {
				continue
			}
			if model == "" {
				model = "codex"
			}
			events = append(events, usage.Event{
				Source:    "codex",
				SessionID: sessionID,
				Project:   projectName(cwd, path),
				CWD:       cwd,
				Model:     model,
				Input:     p.Info.Last.Input,
				Output:    p.Info.Last.Output,
				CacheRead: p.Info.Last.Cached,
				Reasoning: p.Info.Last.Reasoning,
				Timestamp: env.Timestamp,
			})
		}
	}
	return events, scanner.Err()
}

func projectName(cwd, path string) string {
	if cwd != "" {
		return filepath.Base(cwd)
	}
	return filepath.Base(filepath.Dir(path))
}
