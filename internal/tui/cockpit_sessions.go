package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

// sessRow is one agent session of aggregated usage for the Sessions ledger:
// which project/engine it belonged to, when it ran and for how long, and the
// tokens/cost it burned. A "session" is one conversation thread the agent wrote
// to a single log (Claude/Codex/Gemini all tag events with a session id).
type sessRow struct {
	id      string
	project string
	source  string
	first   time.Time
	last    time.Time
	totals  usage.Totals
	models  map[string]struct{}
}

// sessionLedger aggregates events into one row per session id, mirroring the
// `cockpit sessions` static report but with the span and engine kept so the TUI
// can show how long each session ran and which agent drove it.
func sessionLedger(events []usage.Event, prices usage.PriceBook) []sessRow {
	bySess := map[string]*sessRow{}
	for _, e := range events {
		k := e.SessionID
		if k == "" {
			k = "unknown"
		}
		r := bySess[k]
		if r == nil {
			r = &sessRow{id: k, models: map[string]struct{}{}, first: e.Timestamp, last: e.Timestamp}
			bySess[k] = r
		}
		if r.project == "" && e.Project != "" {
			r.project = e.Project
		}
		if r.source == "" && e.Source != "" {
			r.source = e.Source
		}
		if !e.Timestamp.IsZero() {
			if r.first.IsZero() || e.Timestamp.Before(r.first) {
				r.first = e.Timestamp
			}
			if e.Timestamp.After(r.last) {
				r.last = e.Timestamp
			}
		}
		r.totals.Events++
		r.totals.Input += e.Input
		r.totals.Output += e.Output
		r.totals.CacheRead += e.CacheRead
		r.totals.CacheCreate += e.CacheCreate
		r.totals.Total += e.TotalTokens()
		r.totals.CostUSD += usage.EstimateCostWith(e, prices)
		if e.Model != "" {
			r.models[e.Model] = struct{}{}
		}
	}
	rows := make([]sessRow, 0, len(bySess))
	for _, r := range bySess {
		rows = append(rows, *r)
	}
	// Stable default order before the view applies the active sort.
	sort.Slice(rows, func(i, j int) bool { return rows[i].last.After(rows[j].last) })
	return rows
}

// sortedSessions returns the session rows ordered by the active sort mode: most
// recent first (date, the default), or tokens / cost descending.
func (m Model) sortedSessions() []sessRow {
	rows := sessionLedger(m.events, m.reportOptions.Pricing)
	switch m.sortMode {
	case 1:
		sort.Slice(rows, func(i, j int) bool { return rows[i].totals.Total > rows[j].totals.Total })
	case 2:
		sort.Slice(rows, func(i, j int) bool { return rows[i].totals.CostUSD > rows[j].totals.CostUSD })
	}
	return rows
}

// shortSession trims a long session id (often a UUID) for a table cell while
// leaving short ids untouched.
func shortSession(id string, width int) string {
	return truncate(id, width)
}

// sessionsView renders the Sessions tab: a table of per-session usage and cost
// with a TOTAL footer. Arrows move a row cursor; enter opens that session's
// per-model breakdown.
func (m Model) sessionsView(width int) string {
	if m.sessPopup {
		return m.sessionDetail(width)
	}
	span := m.dataSpanLabel()
	rows := m.sortedSessions()
	hero := heroPanel("✈ SESSION SUMMARY · "+span, colCyan, width, m.sessionsSummaryBody(rows))
	body := m.sessionsTable(width, m.scroll, m.tableVisible())
	title := fmt.Sprintf("◈ SESSIONS · last %s · row %d/%d · sort %s · ↑↓ enter · s sort",
		span, m.sessSel+1, len(rows), sortLabel(m.sortMode))
	table := panel(title, colCyan, width, body)
	return vstack(hero, table)
}

func (m Model) sessionsSummaryBody(rows []sessRow) string {
	if len(rows) == 0 {
		return labelStyle.Render("no data")
	}
	cur := m.currency()
	var totals usage.Totals
	projects := map[string]struct{}{}
	engines := map[string]struct{}{}
	for _, row := range rows {
		totals.Total += row.totals.Total
		totals.CostUSD += row.totals.CostUSD
		if row.project != "" {
			projects[row.project] = struct{}{}
		}
		if row.source != "" {
			engines[row.source] = struct{}{}
		}
	}
	cells := []string{
		readout("SESSIONS", compact(int64(len(rows))), colCyan),
		readout("TOKENS", compact(totals.Total), colGreen),
		readout("COST", fmt.Sprintf("~%.2f %s", totals.CostUSD, cur), colGreen),
		readout("PROJECTS", compact(int64(len(projects))), colText),
		readout("ENGINES", compact(int64(len(engines))), colText),
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, spread(cells, "   ")...)
}

// sessionDetail renders the per-model breakdown for the selected session (enter
// on the Sessions tab), with session context (engine, project, span) on top.
func (m Model) sessionDetail(width int) string {
	rows := m.sortedSessions()
	if m.sessSel < 0 || m.sessSel >= len(rows) {
		return panel("◈ SESSIONS", colCyan, width, labelStyle.Render("no data"))
	}
	r := rows[m.sessSel]

	var sessEvents []usage.Event
	for _, e := range m.events {
		k := e.SessionID
		if k == "" {
			k = "unknown"
		}
		if k == r.id {
			sessEvents = append(sessEvents, e)
		}
	}

	proj := r.project
	if proj == "" {
		proj = "—"
	}
	ctx := lipgloss.JoinHorizontal(lipgloss.Top, spread([]string{
		readout("ENGINE", strings.ToUpper(r.source), agentColor(r.source)),
		readout("PROJECT", truncate(proj, 22), colCyan),
		readout("STARTED", r.first.Format("01-02 15:04"), colText),
		readout("ACTIVE", fmtDur(r.last.Sub(r.first)), colAmber),
	}, "   ")...)

	body := lipgloss.JoinVertical(lipgloss.Left, ctx, "", m.usageDetailBody(sessEvents))
	header := fmt.Sprintf("⤢ SESSION · %s   ·   esc back", shortSession(r.id, 24))
	return heroPanel(header, colCyan, width, body)
}

// sessionsTable lists sessions in the active sort order: id, project, engine,
// when it started, how long it stayed active, tokens, and cost, with a grand
// total footer over every session in range.
func (m Model) sessionsTable(width, offset, limit int) string {
	rows := m.sortedSessions()
	if len(rows) == 0 {
		return labelStyle.Render("no data")
	}
	cur := m.currency()

	inner := width - 6
	if inner < 40 {
		inner = 40
	}
	const idW, projW, agentW, startW, durW, numW, costW = 10, 18, 7, 12, 7, 9, 12
	modelsW := inner - idW - projW - agentW - startW - durW - numW - costW - 7 // 7 single-space gaps
	if modelsW < 8 {
		modelsW = 8
	}
	line := func(id, proj, agent, start, dur, tok, cost, models string) string {
		return fmt.Sprintf("%-*s %-*s %-*s %-*s %*s %*s %*s %-*s",
			idW, truncate(id, idW), projW, truncate(proj, projW), agentW, truncate(agent, agentW),
			startW, start, durW, dur, numW, tok, costW, cost, modelsW, truncate(models, modelsW))
	}

	var b strings.Builder
	b.WriteString(labelStyle.Render(line("SESSION", "PROJECT", "ENGINE", "STARTED", "ACTIVE", "TOKENS", "COST", "MODELS")))
	b.WriteByte('\n')

	shown := rows
	if offset > 0 && offset < len(shown) {
		shown = shown[offset:]
	} else if offset >= len(shown) {
		shown = nil
	}
	if limit > 0 && len(shown) > limit {
		shown = shown[:limit]
	}
	for i, r := range shown {
		proj := r.project
		if proj == "" {
			proj = "—"
		}
		row := line(
			shortSession(r.id, idW), proj, r.source,
			r.first.Format("01-02 15:04"),
			fmtDur(r.last.Sub(r.first)),
			compact(r.totals.Total),
			fmt.Sprintf("~%.2f %s", r.totals.CostUSD, cur),
			dayModelList(r.models),
		)
		st := lipgloss.NewStyle().Foreground(colText)
		if offset+i == m.sessSel { // cursor row
			st = st.Reverse(true)
		}
		b.WriteString(st.Render(row))
		b.WriteByte('\n')
	}

	// Grand total over every session in range (not just the shown slice).
	var g usage.Totals
	for _, r := range rows {
		g.Total += r.totals.Total
		g.CostUSD += r.totals.CostUSD
	}
	note := ""
	if limit > 0 && len(rows) > limit {
		note = fmt.Sprintf("%d sessions", len(rows))
	}
	total := line("TOTAL", note, "", "", "", compact(g.Total), fmt.Sprintf("~%.2f %s", g.CostUSD, cur), "")
	b.WriteString(lipgloss.NewStyle().Foreground(colText).Bold(true).Render(total))
	return b.String()
}
