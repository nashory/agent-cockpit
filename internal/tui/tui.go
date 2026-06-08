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
	insights
	activity
	calendar
	numViews
)

type Model struct {
	events          []usage.Event
	view            view
	reportOptions   report.Options
	refreshInterval time.Duration
	reload          func() ([]usage.Event, error)
	lastRefresh     time.Time
	loading         bool
	width           int
	height          int
	compact         bool           // compact (light) vs expert (dense) layout
	ins             usage.Insights // derived once per data load, not per render
	err             error
}

type Options struct {
	Report          report.Options
	RefreshInterval time.Duration
	Reload          func() ([]usage.Event, error)
}

type tickMsg time.Time

type loadedMsg struct {
	events []usage.Event
	err    error
}

func New(events []usage.Event, opts Options) Model {
	m := Model{
		events:          events,
		reportOptions:   opts.Report,
		refreshInterval: opts.RefreshInterval,
		reload:          opts.Reload,
	}
	if len(events) > 0 {
		m.lastRefresh = time.Now()
		m.ins = usage.ComputeInsights(events, opts.Report.Pricing)
	} else if opts.Reload != nil {
		// No data yet: render immediately and load in the background.
		m.loading = true
	}
	return m
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.loading {
		cmds = append(cmds, m.loadCmd())
	}
	if m.refreshInterval > 0 && m.reload != nil {
		cmds = append(cmds, tick(m.refreshInterval))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			return m.startRefresh()
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
		case "6":
			m.view = insights
		case "7":
			m.view = activity
		case "8":
			m.view = calendar
		case "e":
			m.compact = !m.compact
		case "tab", "right", "j":
			m.view = (m.view + 1) % numViews
		case "shift+tab", "left", "k":
			m.view = (m.view + numViews - 1) % numViews
		}
	case loadedMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.events = msg.events
			m.lastRefresh = time.Now()
			m.ins = usage.ComputeInsights(m.events, m.reportOptions.Pricing)
		}
		return m, nil
	case tickMsg:
		refreshed, cmd := m.startRefresh()
		next := tick(m.refreshInterval)
		if cmd != nil {
			return refreshed, tea.Batch(cmd, next)
		}
		return refreshed, next
	}
	return m, nil
}

func (m Model) View() string {
	renderCompact = m.compact // render-context flag for panel/heroPanel/vstack
	sidebar := lipgloss.NewStyle().Width(18).Padding(1, 2).Foreground(lipgloss.Color("244")).Render(m.sidebar())
	content := lipgloss.NewStyle().Padding(1, 2).Render(m.content())
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content) + "\n"
}

func (m Model) sidebar() string {
	items := []string{"1 Overview", "2 Agents", "3 Models", "4 Trends", "5 Speed", "6 Insights", "7 Activity", "8 Calendar"}
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
	if m.loading {
		status = "loading…"
	}
	mode := "expert"
	if m.compact {
		mode = "compact"
	}
	return "agent-cockpit\nac\n" + status + "\nmode " + mode + "\n\n" + lipgloss.JoinVertical(lipgloss.Left, items...) + "\n\ne mode\nr refresh\nq quit"
}

// contentWidth is the drawable width to the right of the sidebar, accounting
// for the sidebar (18) and the content area's horizontal padding (4).
func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 100
	}
	w := m.width - 22 - 4
	if w < 40 {
		w = 40
	}
	return w
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
	if m.loading && len(m.events) == 0 {
		fmt.Fprintln(&b, "Loading local agent logs…")
		return b.String()
	}
	if m.err != nil && len(m.events) == 0 {
		fmt.Fprintln(&b, alertStyle.Render(" FAILED TO LOAD LOGS "))
		fmt.Fprintf(&b, "\n%v\n\n", m.err)
		fmt.Fprintln(&b, labelStyle.Render("press r to retry · q to quit"))
		return b.String()
	}
	w := m.contentWidth()
	switch m.view {
	case overview:
		fmt.Fprint(&b, m.overview(w))
	case agents:
		fmt.Fprint(&b, m.agentsView(w))
	case models:
		fmt.Fprint(&b, m.modelsView(w))
	case trends:
		fmt.Fprint(&b, m.trendsView(w))
	case speed:
		fmt.Fprint(&b, m.speedView(w))
	case insights:
		fmt.Fprint(&b, m.insightsView(w))
	case activity:
		fmt.Fprint(&b, m.activityView(w))
	case calendar:
		fmt.Fprint(&b, m.calendarView(w))
	default:
		fmt.Fprintln(&b, "Unknown view")
	}
	return b.String()
}

// startRefresh kicks off a background reload without blocking the UI. A reload
// already in flight is left to finish so ticks cannot pile up.
func (m Model) startRefresh() (tea.Model, tea.Cmd) {
	if m.reload == nil || m.loading {
		return m, nil
	}
	m.loading = true
	return m, m.loadCmd()
}

func (m Model) loadCmd() tea.Cmd {
	reload := m.reload
	return func() tea.Msg {
		events, err := reload()
		return loadedMsg{events: events, err: err}
	}
}

func tick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
