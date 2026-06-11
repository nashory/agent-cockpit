package builtin

import (
	"github.com/nashory/agent-cockpit/internal/source"
	"github.com/nashory/agent-cockpit/internal/source/claude"
	"github.com/nashory/agent-cockpit/internal/source/codex"
	"github.com/nashory/agent-cockpit/internal/source/gemini"
)

func init() {
	source.Register(claude.Source{})
	source.Register(codex.Source{})
	source.Register(gemini.Source{})
}
