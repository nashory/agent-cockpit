package builtin

import (
	"github.com/nashory/agent-cockpit/internal/source"
	"github.com/nashory/agent-cockpit/internal/source/claude"
	"github.com/nashory/agent-cockpit/internal/source/codex"
	"github.com/nashory/agent-cockpit/internal/source/gemini"
	"github.com/nashory/agent-cockpit/internal/source/opencode"
)

func init() {
	source.Register(claude.Source{})
	source.Register(codex.Source{})
	source.Register(gemini.Source{})
	source.Register(opencode.Source{})
}
