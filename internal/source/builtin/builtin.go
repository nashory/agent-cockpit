package builtin

import (
	"github.com/nashory/agent-cockpit/internal/source"
	"github.com/nashory/agent-cockpit/internal/source/amp"
	"github.com/nashory/agent-cockpit/internal/source/claude"
	"github.com/nashory/agent-cockpit/internal/source/codebuff"
	"github.com/nashory/agent-cockpit/internal/source/codex"
	"github.com/nashory/agent-cockpit/internal/source/copilot"
	"github.com/nashory/agent-cockpit/internal/source/gemini"
	"github.com/nashory/agent-cockpit/internal/source/goose"
	"github.com/nashory/agent-cockpit/internal/source/kilo"
	"github.com/nashory/agent-cockpit/internal/source/kimi"
	"github.com/nashory/agent-cockpit/internal/source/opencode"
	"github.com/nashory/agent-cockpit/internal/source/qwen"
)

func init() {
	source.Register(claude.Source{})
	source.Register(codex.Source{})
	source.Register(gemini.Source{})
	source.Register(opencode.Source{})
	source.Register(amp.Source{})
	source.Register(copilot.Source{})
	source.Register(kimi.Source{})
	source.Register(qwen.Source{})
	source.Register(codebuff.Source{})
	source.Register(kilo.Source{})
	source.Register(goose.Source{})
}
