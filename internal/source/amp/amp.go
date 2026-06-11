package amp

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	sourcepkg "github.com/nashory/agent-cockpit/internal/source"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type Source struct{}

func (Source) Name() string { return "amp" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	return sourcepkg.CollectFiles(ctx, cfg, Source{})
}

func (Source) Roots(cfg config.Config) []string {
	if raw := os.Getenv("AMP_DATA_DIR"); raw != "" {
		return splitEnvPaths(raw)
	}
	return cfg.Paths.Amp
}

func (Source) Match(path string) bool {
	if !strings.HasSuffix(path, ".json") {
		return false
	}
	for _, part := range strings.FieldsFunc(filepath.ToSlash(path), func(r rune) bool { return r == '/' }) {
		if part == "threads" {
			return true
		}
	}
	return false
}

func (Source) Parse(path string, r io.Reader) ([]usage.Event, error) {
	return Parse(r)
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

type threadFile struct {
	ID          string      `json:"id"`
	Messages    []message   `json:"messages"`
	UsageLedger usageLedger `json:"usageLedger"`
}

type usageLedger struct {
	Events []ledgerEvent `json:"events"`
}

type ledgerEvent struct {
	ID          string `json:"id"`
	Timestamp   string `json:"timestamp"`
	Model       string `json:"model"`
	ToMessageID int64  `json:"toMessageId"`
	Tokens      struct {
		Input  int64 `json:"input"`
		Output int64 `json:"output"`
		Total  int64 `json:"total"`
	} `json:"tokens"`
}

type message struct {
	Role      string     `json:"role"`
	Timestamp string     `json:"timestamp"`
	Model     string     `json:"model"`
	MessageID messageID  `json:"messageId"`
	Usage     messageUse `json:"usage"`
}

type messageUse struct {
	Model                    string `json:"model"`
	Timestamp                string `json:"timestamp"`
	InputTokens              int64  `json:"inputTokens"`
	OutputTokens             int64  `json:"outputTokens"`
	CacheCreationInputTokens int64  `json:"cacheCreationInputTokens"`
	CacheReadInputTokens     int64  `json:"cacheReadInputTokens"`
	TotalTokens              int64  `json:"totalTokens"`
}

type messageID struct {
	text string
	int  int64
}

func (id *messageID) UnmarshalJSON(b []byte) error {
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		id.int = n
		id.text = strconv.FormatInt(n, 10)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	id.text = s
	id.int, _ = strconv.ParseInt(s, 10, 64)
	return nil
}

func Parse(r io.Reader) ([]usage.Event, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var thread threadFile
	if err := json.Unmarshal(body, &thread); err != nil {
		return nil, err
	}
	if strings.TrimSpace(thread.ID) == "" {
		return nil, nil
	}
	if len(thread.UsageLedger.Events) > 0 {
		return parseLedger(thread), nil
	}
	return parseMessages(thread), nil
}

func parseLedger(thread threadFile) []usage.Event {
	cacheByMessageID := map[int64][2]int64{}
	for _, msg := range thread.Messages {
		if msg.Role != "assistant" || msg.MessageID.int == 0 {
			continue
		}
		cacheByMessageID[msg.MessageID.int] = [2]int64{
			nonNegative(msg.Usage.CacheCreationInputTokens),
			nonNegative(msg.Usage.CacheReadInputTokens),
		}
	}

	var events []usage.Event
	for _, item := range thread.UsageLedger.Events {
		if item.Model == "" {
			continue
		}
		ts, ok := parseTimestamp(item.Timestamp)
		if !ok {
			continue
		}
		cache := cacheByMessageID[item.ToMessageID]
		input := nonNegative(item.Tokens.Input)
		output := nonNegative(item.Tokens.Output)
		cacheCreate := cache[0]
		cacheRead := cache[1]
		total := nonNegative(item.Tokens.Total)
		applyTotalFallback(&output, input+output+cacheCreate+cacheRead, total)
		if input+output+cacheCreate+cacheRead == 0 {
			continue
		}
		events = append(events, usage.Event{
			Source:      "amp",
			SessionID:   thread.ID,
			Project:     "amp",
			Model:       item.Model,
			Input:       input,
			Output:      output,
			CacheRead:   cacheRead,
			CacheCreate: cacheCreate,
			Timestamp:   ts,
		})
	}
	return events
}

func parseMessages(thread threadFile) []usage.Event {
	var events []usage.Event
	for _, msg := range thread.Messages {
		if msg.Role != "assistant" {
			continue
		}
		model := firstNonEmpty(msg.Usage.Model, msg.Model)
		if model == "" {
			continue
		}
		tsText := firstNonEmpty(msg.Usage.Timestamp, msg.Timestamp)
		ts, ok := parseTimestamp(tsText)
		if !ok {
			continue
		}
		input := nonNegative(msg.Usage.InputTokens)
		output := nonNegative(msg.Usage.OutputTokens)
		cacheCreate := nonNegative(msg.Usage.CacheCreationInputTokens)
		cacheRead := nonNegative(msg.Usage.CacheReadInputTokens)
		total := nonNegative(msg.Usage.TotalTokens)
		applyTotalFallback(&output, input+output+cacheCreate+cacheRead, total)
		if input+output+cacheCreate+cacheRead == 0 {
			continue
		}
		events = append(events, usage.Event{
			Source:      "amp",
			SessionID:   thread.ID,
			Project:     "amp",
			Model:       model,
			Input:       input,
			Output:      output,
			CacheRead:   cacheRead,
			CacheCreate: cacheCreate,
			Timestamp:   ts,
		})
	}
	return events
}

func parseTimestamp(text string) (time.Time, bool) {
	ts, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func applyTotalFallback(output *int64, known, total int64) {
	if total > known {
		*output += total - known
	}
}

func nonNegative(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
