package source_test

import (
	"bufio"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nashory/agent-cockpit/internal/benchdata"
	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/source"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type benchmarkAdapter struct {
	root string
}

func (a benchmarkAdapter) Name() string { return "benchmark" }

func (a benchmarkAdapter) Roots(config.Config) []string { return []string{a.root} }

func (a benchmarkAdapter) Match(path string) bool { return strings.HasSuffix(path, ".jsonl") }

func (a benchmarkAdapter) Parse(path string, r io.Reader) ([]usage.Event, error) {
	scanner := bufio.NewScanner(r)
	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	var out []usage.Event
	for scanner.Scan() {
		out = append(out, usage.Event{
			Source:    "benchmark",
			SessionID: sessionID,
			Model:     "claude-sonnet-4-5",
			Input:     1,
			Output:    1,
		})
	}
	return out, scanner.Err()
}

func BenchmarkCollectFiles(b *testing.B) {
	root := b.TempDir()
	if err := benchdata.WriteLineFixture(root, 1_000, 10); err != nil {
		b.Fatal(err)
	}
	adapter := benchmarkAdapter{root: root}
	cfg := config.Config{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events, err := source.CollectFiles(context.Background(), cfg, adapter)
		if err != nil {
			b.Fatal(err)
		}
		if len(events) != 10_000 {
			b.Fatalf("collected %d events, want 10000", len(events))
		}
	}
}
