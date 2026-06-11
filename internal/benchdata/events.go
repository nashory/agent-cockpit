package benchdata

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nashory/agent-cockpit/internal/usage"
)

func Events(n int) []usage.Event {
	out := make([]usage.Event, 0, n)
	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	models := []string{"claude-sonnet-4-5", "claude-opus-4-8", "gpt-5-codex", "gemini-2.5-pro"}
	sources := []string{"claude", "claude", "codex", "gemini"}
	projects := []string{"agent-cockpit", "infra-tools", "billing-ui", "docs"}
	for i := 0; i < n; i++ {
		idx := i % len(models)
		out = append(out, usage.Event{
			Source:      sources[idx],
			SessionID:   "session-" + strconv.Itoa(i%100),
			Project:     projects[i%len(projects)],
			CWD:         "/tmp/" + projects[i%len(projects)],
			Model:       models[idx],
			Input:       int64(100 + i%50),
			Output:      int64(25 + i%20),
			CacheRead:   int64(i % 10),
			CacheCreate: int64(i % 7),
			Reasoning:   int64(i % 5),
			Timestamp:   base.Add(-time.Duration(i%10080) * time.Minute),
		})
	}
	return out
}

func WriteLineFixture(root string, files, eventsPerFile int) error {
	for i := 0; i < files; i++ {
		var b strings.Builder
		for j := 0; j < eventsPerFile; j++ {
			fmt.Fprintf(&b, "%d,%d\n", i, j)
		}
		path := filepath.Join(root, fmt.Sprintf("project-%03d", i%32), fmt.Sprintf("session-%05d.jsonl", i))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	return nil
}
