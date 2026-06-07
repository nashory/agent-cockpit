package tui

import (
	"bytes"
	"fmt"
	"time"

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
	speed
)

type Model struct {
	events          []usage.Event
	view            view
	reportOptions   report.Options
	refreshInterval time.Duration
	reload          func() ([]usage.Event, error)
	lastRefresh     time.Time
	err             error
}

type Options struct {
	Report          report.Options
	RefreshInterval time.Duration
	Reload          func() ([]usage.Event, error)
}

type tickMsg time.Time

func New(events []usage.Event, opts Options) Model {
	return Model{
		events:          events,
		reportOptions:   opts.Report,
		refreshInterval: opts.RefreshInterval,
		reload:          opts.Reload,
		lastRefresh:     time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	if m.refreshInterval > 0 && m.reload != nil {
		return tick(m.refreshInterval)
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			return m.refresh()
		case "1":
			m.view = overview
		case "2":
			m.view = agents
		case "3":
			m.view = models
		case "4":
			m.view = trends
		case "5":
			m.view = speed
		case "tab", "right", "j":
			m.view = (m.view + 1) % 5
		case "shift+tab", "left", "k":
			m.view = (m.view + 4) % 5
		}
	case tickMsg:
		next := tick(m.refreshInterval)
		refreshed, cmd := m.refresh()
		if cmd != nil {
			return refreshed, tea.Batch(cmd, next)
		}
		return refreshed, next
	}
	return m, nil
}

func (m Model) View() string {
	sidebar := lipgloss.NewStyle().Width(18).Padding(1, 2).Foreground(lipgloss.Color("244")).Render(m.sidebar())
	content := lipgloss.NewStyle().Padding(1, 2).Render(m.content())
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content) + "\n"
}

func (m Model) sidebar() string {
	items := []string{"1 Overview", "2 Agents", "3 Models", "4 Trends", "5 Speed"}
	for i := range items {
		if view(i) == m.view {
			items[i] = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render("> " + items[i])
		} else {
			items[i] = "  " + items[i]
		}
	}
	status := "snapshot"
	if m.refreshInterval > 0 {
		status = "live " + m.refreshInterval.String()
	}
	return "agent-cockpit\nac\n" + status + "\n\n" + lipgloss.JoinVertical(lipgloss.Left, items...) + "\n\nr refresh\nq quit"
}

func (m Model) content() string {
	var b bytes.Buffer
	if !m.lastRefresh.IsZero() {
		fmt.Fprintf(&b, "Updated %s\n", m.lastRefresh.Format("15:04:05"))
	}
	if m.err != nil {
		fmt.Fprintf(&b, "Error: %v\n", m.err)
	}
	fmt.Fprintln(&b)
	switch m.view {
	case overview:
		report.Overview(&b, "Overview", m.events, m.reportOptions)
	case agents:
		report.Buckets(&b, "Agents", usage.GroupByWith(m.events, m.reportOptions.Pricing, func(e usage.Event) string { return e.Source }), 20, m.reportOptions)
	case models:
		report.Buckets(&b, "Models", usage.GroupByWith(m.events, m.reportOptions.Pricing, func(e usage.Event) string { return e.Model }), 20, m.reportOptions)
	case trends:
		report.Trend(&b, m.events, 30, m.reportOptions)
	case speed:
		report.Speed(&b, m.events, 20)
	default:
		fmt.Fprintln(&b, "Unknown view")
	}
	return b.String()
}

func (m Model) refresh() (tea.Model, tea.Cmd) {
	if m.reload == nil {
		return m, nil
	}
	events, err := m.reload()
	m.err = err
	if err == nil {
		m.events = events
		m.lastRefresh = time.Now()
	}
	return m, nil
}

func tick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
