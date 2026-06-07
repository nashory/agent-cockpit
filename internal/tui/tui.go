package tui

import (
	"bytes"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/report"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type view int

const (
	overview view = iota
	agents
	models
	trends
)

type Model struct {
	events []usage.Event
	view   view
}

func New(events []usage.Event) Model {
	return Model{events: events}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "1":
		m.view = overview
	case "2":
		m.view = agents
	case "3":
		m.view = models
	case "4":
		m.view = trends
	case "tab", "right", "j":
		m.view = (m.view + 1) % 4
	case "shift+tab", "left", "k":
		m.view = (m.view + 3) % 4
	}
	return m, nil
}

func (m Model) View() string {
	sidebar := lipgloss.NewStyle().Width(18).Padding(1, 2).Foreground(lipgloss.Color("244")).Render(m.sidebar())
	content := lipgloss.NewStyle().Padding(1, 2).Render(m.content())
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content) + "\n"
}

func (m Model) sidebar() string {
	items := []string{"1 Overview", "2 Agents", "3 Models", "4 Trends"}
	for i := range items {
		if view(i) == m.view {
			items[i] = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render("> " + items[i])
		} else {
			items[i] = "  " + items[i]
		}
	}
	return "agent-cockpit\nac\n\n" + lipgloss.JoinVertical(lipgloss.Left, items...) + "\n\nq quit"
}

func (m Model) content() string {
	var b bytes.Buffer
	switch m.view {
	case overview:
		report.Overview(&b, "Overview", m.events)
	case agents:
		report.Buckets(&b, "Agents", usage.GroupBy(m.events, func(e usage.Event) string { return e.Source }), 20)
	case models:
		report.Buckets(&b, "Models", usage.GroupBy(m.events, func(e usage.Event) string { return e.Model }), 20)
	case trends:
		report.Trend(&b, m.events, 30)
	default:
		fmt.Fprintln(&b, "Unknown view")
	}
	return b.String()
}
