package source

import (
	"context"

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
	var all []usage.Event
	for _, src := range All() {
		events, err := src.Collect(ctx, cfg)
		if err != nil {
			return nil, err
		}
		all = append(all, events...)
	}
	return all, nil
}
