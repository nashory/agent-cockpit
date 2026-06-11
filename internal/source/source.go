package source

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/scan"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type Source interface {
	Name() string
	Collect(context.Context, config.Config) ([]usage.Event, error)
}

type FileAdapter interface {
	Name() string
	Roots(config.Config) []string
	Match(path string) bool
	Parse(path string, r io.Reader) ([]usage.Event, error)
}

var (
	registryMu sync.RWMutex
	registry   []Source
	registered = map[string]bool{}
)

func Register(src Source) {
	if src == nil {
		panic("source: cannot register nil source")
	}
	name := src.Name()
	if name == "" {
		panic("source: cannot register unnamed source")
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if registered[name] {
		panic(fmt.Sprintf("source: source %q already registered", name))
	}
	registered[name] = true
	registry = append(registry, src)
}

func All() []Source {
	registryMu.RLock()
	defer registryMu.RUnlock()
	srcs := make([]Source, len(registry))
	copy(srcs, registry)
	return srcs
}

func CollectFiles(ctx context.Context, cfg config.Config, adapter FileAdapter) ([]usage.Event, error) {
	if adapter == nil {
		return nil, fmt.Errorf("source: nil file adapter")
	}
	return scan.Parallel(ctx, adapter.Roots(cfg), adapter.Match, func(path string) ([]usage.Event, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return adapter.Parse(path, f)
	})
}

func Collect(ctx context.Context, cfg config.Config) ([]usage.Event, error) {
	srcs := All()
	results := make([][]usage.Event, len(srcs))
	errs := make([]error, len(srcs))

	var wg sync.WaitGroup
	for i, src := range srcs {
		wg.Add(1)
		go func(i int, src Source) {
			defer wg.Done()
			results[i], errs[i] = src.Collect(ctx, cfg)
		}(i, src)
	}
	wg.Wait()

	var all []usage.Event
	for i := range srcs {
		if errs[i] != nil {
			return nil, errs[i]
		}
		all = append(all, results[i]...)
	}
	return all, nil
}
