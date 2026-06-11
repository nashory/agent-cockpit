package copilot

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/usage"
)

const fileExporterPathEnv = "COPILOT_OTEL_FILE_EXPORTER_PATH"

type Source struct{}

func (Source) Name() string { return "copilot" }

func (Source) Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	paths, err := discover(ctx, cfg)
	if err != nil {
		return nil, err
	}
	var events []usage.Event
	for _, path := range paths {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		parsed, err := ParseFile(path)
		if err != nil {
			continue
		}
		events = append(events, parsed...)
	}
	return events, nil
}

func discover(ctx context.Context, cfg config.Config) ([]string, error) {
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for _, root := range cfg.Paths.Copilot {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if strings.HasSuffix(root, ".jsonl") {
				add(root)
			}
			continue
		}
		err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !d.IsDir() && strings.HasSuffix(path, ".jsonl") {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if path := strings.TrimSpace(os.Getenv(fileExporterPathEnv)); path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			add(path)
		}
	}
	sort.Strings(paths)
	return paths, nil
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
	var records []record
	for scanner.Scan() {
		line := scanner.Bytes()
		if !strings.Contains(string(line), `"attributes"`) {
			continue
		}
		var rec record
		if err := json.Unmarshal(line, &rec); err == nil && rec.Attributes != nil {
			records = append(records, rec)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	fallback := fileModifiedTimestamp(path)
	contexts := traceContexts(records)
	candidates := make([]candidate, 0, len(records))
	for i, rec := range records {
		if cand, ok := toCandidate(rec, i, fallback, contexts); ok {
			candidates = append(candidates, cand)
		}
	}
	sets := candidateSetsFor(candidates)
	var events []usage.Event
	for _, cand := range candidates {
		if !shouldEmit(cand, sets) {
			continue
		}
		events = append(events, usage.Event{
			Source:      "copilot",
			SessionID:   cand.sessionID,
			Project:     "copilot",
			Model:       cand.model,
			Input:       cand.input,
			Output:      cand.output,
			CacheRead:   cand.cacheRead,
			CacheCreate: cand.cacheCreate,
			Reasoning:   cand.reasoning,
			Timestamp:   cand.timestamp,
		})
	}
	return events, nil
}

type record struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	TraceID     string         `json:"traceId"`
	SpanID      string         `json:"spanId"`
	SpanContext map[string]any `json:"spanContext"`
	Body        string         `json:"body"`
	BodyAlt     string         `json:"_body"`
	StartTime   any            `json:"startTime"`
	EndTime     any            `json:"endTime"`
	HRTime      any            `json:"hrTime"`
	HRTimeAlt   any            `json:"_hrTime"`
	Time        any            `json:"time"`
	Timestamp   any            `json:"timestamp"`
	Observed    any            `json:"observedTimestamp"`
	TimeNanos   any            `json:"timeUnixNano"`
	Attributes  map[string]any `json:"attributes"`
}

type sourceKind int

const (
	chatSpan sourceKind = iota
	inferenceLog
	agentTurnLog
	agentSummarySpan
)

type traceContext struct {
	model           string
	sessionID       string
	sessionPriority int
}

type candidate struct {
	source      sourceKind
	traceID     string
	responseID  string
	sessionID   string
	model       string
	timestamp   time.Time
	input       int64
	output      int64
	cacheRead   int64
	cacheCreate int64
	reasoning   int64
}

func traceContexts(records []record) map[string]traceContext {
	contexts := map[string]traceContext{}
	for _, rec := range records {
		traceID := rec.traceID()
		if traceID == "" {
			continue
		}
		ctx := contexts[traceID]
		if ctx.model == "" {
			ctx.model = firstAttr(rec.Attributes, modelAttrs...)
		}
		if sessionID, priority := bestSession(rec.Attributes); sessionID != "" && priority > ctx.sessionPriority {
			ctx.sessionID = sessionID
			ctx.sessionPriority = priority
		}
		contexts[traceID] = ctx
	}
	return contexts
}

func toCandidate(rec record, index int, fallback time.Time, contexts map[string]traceContext) (candidate, bool) {
	kind, ok := recordKind(rec)
	if !ok {
		return candidate{}, false
	}
	cacheRead := attrInt(rec.Attributes, "gen_ai.usage.cache_read.input_tokens")
	input := attrInt(rec.Attributes, "gen_ai.usage.input_tokens") - cacheRead
	if input < 0 {
		input = 0
	}
	output := attrInt(rec.Attributes, "gen_ai.usage.output_tokens")
	cacheCreate := firstAttrInt(rec.Attributes, "gen_ai.usage.cache_write.input_tokens", "gen_ai.usage.cache_creation.input_tokens")
	reasoning := firstAttrInt(rec.Attributes, "gen_ai.usage.reasoning.output_tokens", "gen_ai.usage.reasoning_tokens")
	total := firstAttrInt(rec.Attributes, "gen_ai.usage.total_tokens", "gen_ai.usage.total.token_count")
	known := input + output + cacheRead + cacheCreate + reasoning
	if total > known {
		output += total - known
	}
	if input+output+cacheRead+cacheCreate+reasoning == 0 {
		return candidate{}, false
	}
	output += reasoning
	traceID := rec.traceID()
	ctx := contexts[traceID]
	model := firstNonEmpty(firstAttr(rec.Attributes, modelAttrs...), ctx.model, "copilot")
	sessionID, _ := bestSession(rec.Attributes)
	sessionID = firstNonEmpty(sessionID, ctx.sessionID, traceID, "unknown-session")
	return candidate{
		source:      kind,
		traceID:     traceID,
		responseID:  attrString(rec.Attributes, "gen_ai.response.id"),
		sessionID:   sessionID,
		model:       model,
		timestamp:   timestampFromRecord(rec, fallback),
		input:       input,
		output:      output,
		cacheRead:   cacheRead,
		cacheCreate: cacheCreate,
		reasoning:   reasoning,
	}, true
}

func recordKind(rec record) (sourceKind, bool) {
	operation := attrString(rec.Attributes, "gen_ai.operation.name")
	eventName := attrString(rec.Attributes, "event.name")
	body := firstNonEmpty(rec.Body, rec.BodyAlt)
	if isSpan(rec) && (operation == "chat" || strings.HasPrefix(rec.Name, "chat ")) {
		return chatSpan, true
	}
	if isSpan(rec) && (operation == "invoke_agent" || strings.HasPrefix(rec.Name, "invoke_agent ")) {
		return agentSummarySpan, true
	}
	if !isSpan(rec) && (eventName == "gen_ai.client.inference.operation.details" || strings.HasPrefix(body, "GenAI inference:")) {
		return inferenceLog, true
	}
	if !isSpan(rec) && (eventName == "copilot_chat.agent.turn" || strings.HasPrefix(body, "copilot_chat.agent.turn")) {
		return agentTurnLog, true
	}
	return 0, false
}

type candidateSets struct {
	chatTraces         map[string]bool
	inferenceTraces    map[string]bool
	agentTurnTraces    map[string]bool
	chatResponses      map[string]bool
	inferenceResponses map[string]bool
	agentTurnResponses map[string]bool
}

func candidateSetsFor(candidates []candidate) candidateSets {
	return candidateSets{
		chatTraces:         sourceTraceSet(candidates, chatSpan),
		inferenceTraces:    sourceTraceSet(candidates, inferenceLog),
		agentTurnTraces:    sourceTraceSet(candidates, agentTurnLog),
		chatResponses:      sourceResponseSet(candidates, chatSpan),
		inferenceResponses: sourceResponseSet(candidates, inferenceLog),
		agentTurnResponses: sourceResponseSet(candidates, agentTurnLog),
	}
}

func shouldEmit(c candidate, sets candidateSets) bool {
	traceIn := func(values map[string]bool) bool { return c.traceID != "" && values[c.traceID] }
	responseIn := func(values map[string]bool) bool { return c.responseID != "" && values[c.responseID] }
	switch c.source {
	case chatSpan:
		return true
	case inferenceLog:
		return !traceIn(sets.chatTraces) && !responseIn(sets.chatResponses)
	case agentTurnLog:
		return !traceIn(sets.chatTraces) && !traceIn(sets.inferenceTraces) && !responseIn(sets.chatResponses) && !responseIn(sets.inferenceResponses)
	case agentSummarySpan:
		return !traceIn(sets.chatTraces) && !traceIn(sets.inferenceTraces) && !traceIn(sets.agentTurnTraces) && !responseIn(sets.chatResponses) && !responseIn(sets.inferenceResponses) && !responseIn(sets.agentTurnResponses)
	default:
		return false
	}
}

func sourceTraceSet(candidates []candidate, kind sourceKind) map[string]bool {
	out := map[string]bool{}
	for _, cand := range candidates {
		if cand.source == kind && cand.traceID != "" {
			out[cand.traceID] = true
		}
	}
	return out
}

func sourceResponseSet(candidates []candidate, kind sourceKind) map[string]bool {
	out := map[string]bool{}
	for _, cand := range candidates {
		if cand.source == kind && cand.responseID != "" {
			out[cand.responseID] = true
		}
	}
	return out
}

var modelAttrs = []string{"gen_ai.response.model", "gen_ai.request.model"}

var sessionAttrs = []struct {
	key      string
	priority int
}{
	{"gen_ai.conversation.id", 3},
	{"copilot_chat.session_id", 3},
	{"copilot_chat.chat_session_id", 3},
	{"session.id", 3},
	{"github.copilot.interaction_id", 2},
	{"gen_ai.response.id", 1},
}

func isSpan(rec record) bool {
	if rec.Type != "" {
		return rec.Type == "span"
	}
	return rec.Name != "" && (rec.SpanID != "" || rec.TraceID != "" || rec.StartTime != nil || rec.EndTime != nil)
}

func (rec record) traceID() string {
	return firstNonEmpty(rec.TraceID, stringMapValue(rec.SpanContext, "traceId"))
}

func firstAttr(attrs map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := attrString(attrs, key); value != "" {
			return value
		}
	}
	return ""
}

func bestSession(attrs map[string]any) (string, int) {
	var best string
	var priority int
	for _, attr := range sessionAttrs {
		if value := attrString(attrs, attr.key); value != "" && attr.priority > priority {
			best = value
			priority = attr.priority
		}
	}
	return best, priority
}

func attrString(attrs map[string]any, key string) string {
	return stringValue(attrs[key])
}

func attrInt(attrs map[string]any, key string) int64 {
	return intValue(attrs[key])
}

func firstAttrInt(attrs map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value := attrInt(attrs, key); value > 0 {
			return value
		}
	}
	return 0
}

func stringMapValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	return stringValue(values[key])
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func intValue(value any) int64 {
	switch v := value.(type) {
	case float64:
		if v < 0 {
			return 0
		}
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if n < 0 {
			return 0
		}
		return n
	default:
		return 0
	}
}

func timestampFromRecord(rec record, fallback time.Time) time.Time {
	for _, value := range []any{rec.EndTime, rec.StartTime, rec.HRTime, rec.HRTimeAlt, rec.Time} {
		if ts, ok := timestampFromParts(value); ok {
			return ts
		}
	}
	for _, value := range []any{rec.Timestamp, rec.Observed} {
		if ts, ok := timestampFromScalar(value); ok {
			return ts
		}
	}
	if ts, ok := timestampFromNanos(rec.TimeNanos); ok {
		return ts
	}
	return fallback
}

func timestampFromParts(value any) (time.Time, bool) {
	parts, ok := value.([]any)
	if !ok || len(parts) < 2 {
		return time.Time{}, false
	}
	seconds := intValue(parts[0])
	nanos := intValue(parts[1])
	if seconds <= 0 {
		return time.Time{}, false
	}
	return time.Unix(seconds, nanos).UTC(), true
}

func timestampFromScalar(value any) (time.Time, bool) {
	raw := intValue(value)
	if raw <= 0 {
		return time.Time{}, false
	}
	var millis int64
	switch {
	case raw >= 100_000_000_000_000_000:
		millis = raw / 1_000_000
	case raw >= 100_000_000_000_000:
		millis = raw / 1_000
	case raw >= 100_000_000_000:
		millis = raw
	default:
		millis = raw * 1_000
	}
	return time.UnixMilli(millis).UTC(), true
}

func timestampFromNanos(value any) (time.Time, bool) {
	raw := intValue(value)
	if raw <= 0 {
		return time.Time{}, false
	}
	return time.Unix(0, raw).UTC(), true
}

func fileModifiedTimestamp(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime().UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
