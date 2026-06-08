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
	breakdown
	trends
	activity
	numViews
)

type Model struct {
	events          []usage.Event
	view            view
	reportOptions   report.Options
	refreshInterval time.Duration
	reload          func() ([]usage.Event, error)
	fsEvents        <-chan struct{}
	lastRefresh     time.Time
	loading         bool
	width           int
	height          int
	compact         bool           // compact (light) vs expert (dense) layout
	focus           int            // focused widget within the current tab
	zoomed          bool           // fullscreen the focused widget
	calCursor       int            // calendar: selected day, days before today
	ins             usage.Insights // derived once per data load, not per render
	err             error
}

const calMaxCursor = 53*7 - 1

func clampCursor(c int) int {
	if c < 0 {
		return 0
	}
	if c > calMaxCursor {
		return calMaxCursor
	}
	return c
}

type Options struct {
	Report          report.Options
	RefreshInterval time.Duration
	Reload          func() ([]usage.Event, error)
	FSEvents        <-chan struct{} // optional fsnotify-driven refresh signal
}

type tickMsg time.Time

type fsEventMsg struct{}

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
		fsEvents:        opts.FSEvents,
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
	if m.fsEvents != nil {
		cmds = append(cmds, waitForFS(m.fsEvents))
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
		s := msg.String()
		targets := m.zoomTargets()
		n := len(targets)

		// When zoomed into the calendar widget, arrows drive the day cursor.
		if m.zoomed && m.focus < n && targets[m.focus].title == "CALENDAR" {
			switch s {
			case "left", "h":
				m.calCursor = clampCursor(m.calCursor + 7)
				return m, nil
			case "right", "l":
				m.calCursor = clampCursor(m.calCursor - 7)
				return m, nil
			case "up", "k":
				m.calCursor = clampCursor(m.calCursor + 1)
				return m, nil
			case "down", "j":
				m.calCursor = clampCursor(m.calCursor - 1)
				return m, nil
			}
		}

		switch s {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.zoomed = false
		case "enter":
			if n > 0 {
				m.zoomed = true
			}
		case "r":
			return m.startRefresh()
		case "e":
			m.compact = !m.compact
		case "1", "2", "3", "4":
			m.view = view(int(s[0] - '1'))
			m.focus, m.zoomed = 0, false
		case "tab":
			m.view = (m.view + 1) % numViews
			m.focus, m.zoomed = 0, false
		case "shift+tab":
			m.view = (m.view + numViews - 1) % numViews
			m.focus, m.zoomed = 0, false
		case "left", "h", "up", "k":
			if n > 0 {
				m.focus = (m.focus - 1 + n) % n
			}
		case "right", "l", "down", "j":
			if n > 0 {
				m.focus = (m.focus + 1) % n
			}
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
	case fsEventMsg:
		refreshed, cmd := m.startRefresh()
		next := waitForFS(m.fsEvents)
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
	items := []string{"1 Overview", "2 Breakdown", "3 Trends", "4 Activity"}
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
	return "agent-cockpit\nac\n" + status + "\nmode " + mode + "\n\n" + lipgloss.JoinVertical(lipgloss.Left, items...) + "\n\n↑↓ focus\nenter zoom\ne mode\nr refresh\nq quit"
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

// contentHeight is the drawable height for a fullscreen (zoomed) widget.
func (m Model) contentHeight() int {
	if m.height <= 0 {
		return 30
	}
	h := m.height - 8
	if h < 10 {
		h = 10
	}
	return h
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
	targets := m.zoomTargets()
	if len(targets) > 0 && m.focus >= len(targets) {
		m.focus = len(targets) - 1
	}
	if m.zoomed && len(targets) > 0 {
		fmt.Fprint(&b, m.zoomedContent(w, m.contentHeight(), targets))
		return b.String()
	}
	if bar := m.focusBar(targets); bar != "" {
		fmt.Fprintln(&b, clipLines(bar, w))
		fmt.Fprintln(&b)
	}
	switch m.view {
	case overview:
		fmt.Fprint(&b, m.overview(w))
	case breakdown:
		fmt.Fprint(&b, m.breakdownView(w))
	case trends:
		fmt.Fprint(&b, m.trendsView(w))
	case activity:
		fmt.Fprint(&b, m.activityView(w))
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

// waitForFS blocks on the fsnotify signal channel and yields one fsEventMsg,
// re-armed by Update after each event.
func waitForFS(ch <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		if _, ok := <-ch; !ok {
			return nil
		}
		return fsEventMsg{}
	}
}
