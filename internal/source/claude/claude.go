package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type Source struct{}

func (Source) Name() string { return "claude" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	var events []usage.Event
	for _, root := range cfg.ClaudePaths {
		if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
				return nil
			}
			parsed, err := ParseFile(path)
			if err != nil {
				return nil
			}
			events = append(events, parsed...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return events, nil
}

type line struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"sessionId"`
	CWD       string    `json:"cwd"`
	Message   struct {
		Model string `json:"model"`
		Usage struct {
			Input       int64 `json:"input_tokens"`
			Output      int64 `json:"output_tokens"`
			CacheCreate int64 `json:"cache_creation_input_tokens"`
			CacheRead   int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
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
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var events []usage.Event
	for scanner.Scan() {
		var l line
		if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
			continue
		}
		if l.Message.Usage.Input+l.Message.Usage.Output+l.Message.Usage.CacheCreate+l.Message.Usage.CacheRead == 0 {
			continue
		}
		events = append(events, usage.Event{
			Source:      "claude",
			SessionID:   l.SessionID,
			Project:     projectName(l.CWD, path),
			CWD:         l.CWD,
			Model:       l.Message.Model,
			Input:       l.Message.Usage.Input,
			Output:      l.Message.Usage.Output,
			CacheRead:   l.Message.Usage.CacheRead,
			CacheCreate: l.Message.Usage.CacheCreate,
			Timestamp:   l.Timestamp,
		})
	}
	return events, scanner.Err()
}

func projectName(cwd, path string) string {
	if cwd != "" {
		return filepath.Base(cwd)
	}
	dir := filepath.Base(filepath.Dir(path))
	return strings.Trim(dir, "-")
}
