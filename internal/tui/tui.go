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
	daily
	blocks
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
	scroll          int            // row offset for the table tabs (Daily, Blocks)
	daySel          int            // Daily tab: selected row (absolute index, 0 = newest)
	dayPopup        bool           // Daily tab: show the selected day's per-model breakdown
	blkSel          int            // Blocks tab: selected row (absolute index, 0 = newest)
	blkPopup        bool           // Blocks tab: show the selected window's per-model breakdown
	trendSel        int            // Trends TOKENS/COST zoom: selected day index (0=oldest .. trendDays-1=today)
	showHelp        bool           // full-screen keyboard help overlay
	filter          string         // active filter description for the sidebar
	logDirs         []string       // log locations for the empty state
	ins             usage.Insights // derived once per data load, not per render
	err             error
}

const calMaxCursor = 53*7 - 1
const trendDays = 30

func clampCursor(c int) int {
	if c < 0 {
		return 0
	}
	if c > calMaxCursor {
		return calMaxCursor
	}
	return c
}

func clampScroll(c, max int) int {
	if c < 0 {
		return 0
	}
	if c > max {
		return max
	}
	return c
}

// tableTotal is the number of rows the current table tab can show.
func (m Model) tableTotal() int {
	switch m.view {
	case daily:
		return len(dailyLedger(m.events, m.reportOptions.Pricing))
	case blocks:
		return len(usage.SessionBlocks(m.events, m.reportOptions.Pricing, usage.DefaultBlockWindow))
	}
	return 0
}

// tableVisible is how many table rows fit in the current context (zoomed uses
// the full content height; otherwise the tab leaves room for chrome).
func (m Model) tableVisible() int {
	if m.zoomed {
		if v := m.contentHeight() - 4; v > 6 {
			return v
		}
		return 6
	}
	switch m.view {
	case daily:
		if v := m.height - 14; v > 6 {
			return v
		}
		return 6
	case blocks:
		if v := m.height - 16; v > 5 {
			return v
		}
		return 5
	}
	return 0
}

func (m Model) maxScroll() int {
	if mx := m.tableTotal() - m.tableVisible(); mx > 0 {
		return mx
	}
	return 0
}

type Options struct {
	Report          report.Options
	RefreshInterval time.Duration
	Reload          func() ([]usage.Event, error)
	FSEvents        <-chan struct{} // optional fsnotify-driven refresh signal
	Filter          string          // human-readable active filters (source/project/...), shown in the sidebar
	LogDirs         []string        // log locations, shown in the empty state
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
		filter:          opts.Filter,
		logDirs:         opts.LogDirs,
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

		// Help overlay takes priority: ? toggles it, esc / q close it, and while
		// it is open other keys are swallowed so nothing changes underneath.
		if s == "ctrl+c" {
			return m, tea.Quit
		}
		if m.showHelp {
			if s == "?" || s == "esc" || s == "q" {
				m.showHelp = false
			}
			return m, nil
		}
		if s == "?" {
			m.showHelp = true
			return m, nil
		}

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

		// Zoomed into the TOKENS/COST trend chart: left/right move a date cursor.
		if m.zoomed && m.focus < n && (targets[m.focus].title == "TOKENS" || targets[m.focus].title == "COST") {
			switch s {
			case "left", "h", "up", "k":
				if m.trendSel > 0 {
					m.trendSel--
				}
				return m, nil
			case "right", "l", "down", "j":
				if m.trendSel < trendDays-1 {
					m.trendSel++
				}
				return m, nil
			}
		}

		// Daily tab: arrows move a row cursor; enter opens that day's per-model
		// breakdown; esc closes it. The scroll offset follows the cursor.
		if m.view == daily {
			total := m.tableTotal()
			vis := m.tableVisible()
			move := func(d int) {
				m.daySel += d
				if m.daySel < 0 {
					m.daySel = 0
				}
				if total > 0 && m.daySel > total-1 {
					m.daySel = total - 1
				}
				if m.daySel < m.scroll {
					m.scroll = m.daySel
				}
				if vis > 0 && m.daySel >= m.scroll+vis {
					m.scroll = m.daySel - vis + 1
				}
			}
			switch s {
			case "up", "k":
				move(-1)
				return m, nil
			case "down", "j":
				move(1)
				return m, nil
			case "pgup":
				move(-vis)
				return m, nil
			case "pgdown":
				move(vis)
				return m, nil
			case "home", "g":
				move(-total)
				return m, nil
			case "end", "G":
				move(total)
				return m, nil
			case "enter":
				if total > 0 {
					m.dayPopup = true
				}
				return m, nil
			case "esc":
				if m.dayPopup {
					m.dayPopup = false
					return m, nil
				}
			}
		}

		// Blocks tab: arrows move a row cursor; enter opens that window's per-model
		// breakdown; esc closes it. The scroll offset follows the cursor.
		if m.view == blocks {
			total := m.tableTotal()
			vis := m.tableVisible()
			move := func(d int) {
				m.blkSel += d
				if m.blkSel < 0 {
					m.blkSel = 0
				}
				if total > 0 && m.blkSel > total-1 {
					m.blkSel = total - 1
				}
				if m.blkSel < m.scroll {
					m.scroll = m.blkSel
				}
				if vis > 0 && m.blkSel >= m.scroll+vis {
					m.scroll = m.blkSel - vis + 1
				}
			}
			switch s {
			case "up", "k":
				move(-1)
				return m, nil
			case "down", "j":
				move(1)
				return m, nil
			case "pgup":
				move(-vis)
				return m, nil
			case "pgdown":
				move(vis)
				return m, nil
			case "home", "g":
				move(-total)
				return m, nil
			case "end", "G":
				move(total)
				return m, nil
			case "enter":
				if total > 0 {
					m.blkPopup = true
				}
				return m, nil
			case "esc":
				if m.blkPopup {
					m.blkPopup = false
					return m, nil
				}
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
			m.focus, m.zoomed = 0, false // target list changes with the layout
		case "1", "2", "3", "4", "5", "6":
			m.view = view(int(s[0] - '1'))
			m.focus, m.zoomed, m.scroll, m.daySel, m.dayPopup, m.blkSel, m.blkPopup = 0, false, 0, 0, false, 0, false
			m.trendSel = trendDays - 1
		case "tab":
			m.view = (m.view + 1) % numViews
			m.focus, m.zoomed, m.scroll, m.daySel, m.dayPopup, m.blkSel, m.blkPopup = 0, false, 0, 0, false, 0, false
			m.trendSel = trendDays - 1
		case "shift+tab":
			m.view = (m.view + numViews - 1) % numViews
			m.focus, m.zoomed, m.scroll, m.daySel, m.dayPopup, m.blkSel, m.blkPopup = 0, false, 0, 0, false, 0, false
			m.trendSel = trendDays - 1
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
	// Below this the boxed layout cannot render legibly; ask for more room.
	if m.width > 0 && m.width < 60 || m.height > 0 && m.height < 16 {
		return fmt.Sprintf("agent-cockpit needs a larger terminal\n(at least 60x16, this is %dx%d)\n", m.width, m.height)
	}
	sidebar := lipgloss.NewStyle().Width(18).Padding(1, 2).Foreground(lipgloss.Color("244")).Render(m.sidebar())
	content := lipgloss.NewStyle().Padding(1, 2).Render(m.content())
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content) + "\n"
}

func (m Model) sidebar() string {
	items := []string{"1 Overview", "2 Breakdown", "3 Trends", "4 Activity", "5 Daily", "6 Blocks"}
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
	// Context-sensitive primary hint: the same keys do different things per tab.
	nav := "↑↓ focus\nenter zoom"
	switch m.view {
	case daily:
		nav = "↑↓ select\nenter day"
	case blocks:
		nav = "↑↓ select\nenter window"
	}
	hints := nav + "\nesc back\ne mode\nr refresh\n? help\nq quit"
	head := "agent-cockpit\ncockpit\n" + status + "\nmode " + mode
	if m.filter != "" {
		head += "\n" + warnStyle.Render("filtered") + "\n" + labelStyle.Render(m.filter)
	}
	return head + "\n\n" + lipgloss.JoinVertical(lipgloss.Left, items...) + "\n\n" + hints
}

// helpView is the full-screen keyboard reference (toggled with ?).
func (m Model) helpView(width int) string {
	row := func(k, d string) string {
		return labelStyle.Render(fmt.Sprintf("  %-16s ", k)) + lipgloss.NewStyle().Foreground(colText).Render(d)
	}
	lines := []string{
		titleStyle.Render("KEYBOARD"),
		"",
		row("1 - 6", "jump to a tab"),
		row("tab / shift+tab", "next / previous tab"),
		row("e", "toggle expert / compact layout"),
		row("r", "refresh now"),
		row("? ", "toggle this help"),
		row("q / ctrl+c", "quit"),
		"",
		titleStyle.Render("OVERVIEW · BREAKDOWN · TRENDS · ACTIVITY"),
		"",
		row("arrows / hjkl", "move the widget focus"),
		row("enter", "zoom the focused widget fullscreen"),
		row("esc", "leave zoom"),
		row("← →", "on a zoomed TOKENS/COST chart, move the date cursor"),
		row("arrows", "on a zoomed calendar, move the day cursor"),
		"",
		titleStyle.Render("DAILY · BLOCKS"),
		"",
		row("↑↓ / jk", "move the row cursor"),
		row("pgup/pgdn, g/G", "page, jump to top / bottom"),
		row("enter", "open the row's per-model breakdown"),
		row("esc", "close the breakdown"),
		"",
		labelStyle.Render("Costs are estimated from token counts, shown with a leading ~."),
		labelStyle.Render("press ? or esc to close"),
	}
	return heroPanel("✈ HELP", colCyan, width, lipgloss.JoinVertical(lipgloss.Left, lines...))
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
	if !m.loading && len(m.events) == 0 {
		fmt.Fprintln(&b, warnStyle.Render(" NO AGENT LOGS FOUND "))
		fmt.Fprintln(&b)
		if m.filter != "" {
			fmt.Fprintf(&b, "No events matched your filter (%s).\n\n", m.filter)
		} else {
			fmt.Fprintln(&b, "Nothing to show yet. Looked in:")
			if len(m.logDirs) == 0 {
				fmt.Fprintln(&b, labelStyle.Render("  (default Claude Code / Codex / Gemini locations)"))
			}
			for _, d := range m.logDirs {
				fmt.Fprintln(&b, labelStyle.Render("  "+d))
			}
			fmt.Fprintln(&b)
		}
		fmt.Fprintln(&b, labelStyle.Render("Run `cockpit doctor` to see detected paths · r retry · q quit"))
		return b.String()
	}
	w := m.contentWidth()
	if m.showHelp {
		fmt.Fprint(&b, m.helpView(w))
		return b.String()
	}
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
	case daily:
		fmt.Fprint(&b, m.dailyView(w))
	case blocks:
		fmt.Fprint(&b, m.blocksView(w))
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
