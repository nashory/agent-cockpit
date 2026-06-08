// Package scan walks log directories and parses matching files concurrently.
package scan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/nashory/agent-cockpit/internal/usage"
)

// ParseFunc turns a single file into usage events.
type ParseFunc func(path string) ([]usage.Event, error)

// MatchFunc reports whether a walked path should be parsed.
type MatchFunc func(path string) bool

// Parallel walks roots, selects files via match, and parses them across a pool
// of workers sized to the number of CPUs. Per-file parse errors are skipped so
// one corrupt log cannot abort the whole scan. The returned events are not in a
// deterministic order; downstream aggregation does not depend on it.
func Parallel(ctx context.Context, roots []string, match MatchFunc, parse ParseFunc) ([]usage.Event, error) {
	paths, err := collectPaths(ctx, roots, match)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}

	workers := runtime.NumCPU()
	if workers > len(paths) {
		workers = len(paths)
	}

	results := make([][]usage.Event, len(paths))
	indices := make(chan int)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indices {
				if ctx.Err() != nil {
					return
				}
				parsed, perr := parse(paths[i])
				if perr != nil {
					continue
				}
				results[i] = parsed
			}
		}()
	}

feed:
	for i := range paths {
		select {
		case <-ctx.Done():
			break feed
		case indices <- i:
		}
	}
	close(indices)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var all []usage.Event
	for _, r := range results {
		all = append(all, r...)
	}
	return all, nil
}

func collectPaths(ctx context.Context, roots []string, match MatchFunc) ([]string, error) {
	var paths []string
	for _, root := range roots {
		if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() || !match(path) {
				return nil
			}
			paths = append(paths, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}
