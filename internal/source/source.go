package source

import (
	"context"
	"sync"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/source/claude"
	"github.com/nashory/agent-cockpit/internal/source/codex"
	"github.com/nashory/agent-cockpit/internal/source/gemini"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type Source interface {
	Name() string
	Collect(context.Context, config.Config) ([]usage.Event, error)
}

func All() []Source {
	return []Source{
		claude.Source{},
		codex.Source{},
		gemini.Source{},
	}
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
