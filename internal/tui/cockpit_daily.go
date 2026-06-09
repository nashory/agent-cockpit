package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

// dayRow is one calendar day of aggregated usage for the Daily ledger.
type dayRow struct {
	date   string
	totals usage.Totals
	models map[string]struct{}
}

// dailyLedger aggregates events into one row per calendar day, newest first,
// ccusage-style: per-day input/output/cache/total tokens, cost, and the set of
// models touched that day.
func dailyLedger(events []usage.Event, prices usage.PriceBook) []dayRow {
	byDay := map[string]*dayRow{}
	for _, e := range events {
		if e.Timestamp.IsZero() {
			continue
		}
		k := e.Timestamp.Format("2006-01-02")
		r := byDay[k]
		if r == nil {
			r = &dayRow{date: k, models: map[string]struct{}{}}
			byDay[k] = r
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
	rows := make([]dayRow, 0, len(byDay))
	for _, r := range byDay {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].date > rows[j].date }) // newest first
	return rows
}

// shortModel trims a provider/version-heavy model id to something readable in a
// table cell: "claude-opus-4-8" -> "opus-4-8", "gpt-5-codex" -> "gpt-5-codex".
func shortModel(m string) string {
	m = strings.TrimPrefix(m, "claude-")
	return m
}

func dayModelList(set map[string]struct{}) string {
	names := make([]string, 0, len(set))
	for k := range set {
		names = append(names, shortModel(k))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// dailyView renders the Daily ledger tab: a table of per-day usage and cost with
// a TOTAL footer, the way ccusage prints its daily report. Arrows move a row
// cursor; enter opens that day's per-model breakdown.
func (m Model) dailyView(width int) string {
	if m.dayPopup {
		return m.dayDetail(width)
	}
	span := m.dataSpanLabel()
	body := m.ledgerTable(width, m.scroll, m.tableVisible())
	title := "◈ DAILY · last " + span + "  · ↑↓ select · enter day"
	return panel(title, colCyan, width, body)
}

// dayDetail renders the per-model breakdown for the selected day (enter on the
// Daily tab), the drill-down ccusage shows on its daily rows.
func (m Model) dayDetail(width int) string {
	rows := dailyLedger(m.events, m.reportOptions.Pricing)
	if m.daySel < 0 || m.daySel >= len(rows) {
		return panel("◈ DAILY", colCyan, width, labelStyle.Render("no data"))
	}
	date := rows[m.daySel].date

	var dayEvents []usage.Event
	for _, e := range m.events {
		if !e.Timestamp.IsZero() && e.Timestamp.Format("2006-01-02") == date {
			dayEvents = append(dayEvents, e)
		}
	}
	weekday := ""
	if t, err := time.Parse("2006-01-02", date); err == nil {
		weekday = t.Format("Mon")
	}
	header := fmt.Sprintf("⤢ DAILY · %s %s   ·   esc back", date, weekday)
	return heroPanel(header, colCyan, width, m.usageDetailBody(dayEvents))
}

// scrollHint shows "· 31-60 / 190 ↑↓" when a table has more rows than fit.
func scrollHint(offset, visible, total int) string {
	if total <= visible {
		return ""
	}
	from := offset + 1
	to := offset + visible
	if to > total {
		to = total
	}
	return fmt.Sprintf(" · %d-%d / %d  ↑↓", from, to, total)
}

// ledgerTable formats the daily ledger to the given content width, showing
// `limit` rows starting at `offset`. The TOTAL row always reflects every day.
func (m Model) ledgerTable(width, offset, limit int) string {
	rows := dailyLedger(m.events, m.reportOptions.Pricing)
	if len(rows) == 0 {
		return labelStyle.Render("no data")
	}
	cur := m.currency()

	inner := width - 6
	if inner < 40 {
		inner = 40
	}
	const dateW, numW, totW, costW = 10, 9, 11, 12
	modelsW := inner - dateW - numW*3 - totW - costW - 6 // 6 single-space gaps
	if modelsW < 8 {
		modelsW = 8
	}

	line := func(date, in, out, cache, tot, cost, models string) string {
		return fmt.Sprintf("%-*s %*s %*s %*s %*s %*s %-*s",
			dateW, truncate(date, dateW),
			numW, in, numW, out, numW, cache,
			totW, tot, costW, cost,
			modelsW, truncate(models, modelsW))
	}

	var b strings.Builder
	b.WriteString(labelStyle.Render(line("DATE", "INPUT", "OUTPUT", "CACHE", "TOTAL", "COST", "MODELS")))
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
		t := r.totals
		row := line(
			r.date,
			compact(t.Input), compact(t.Output), compact(t.CacheRead+t.CacheCreate),
			compact(t.Total), fmt.Sprintf("~%.2f %s", t.CostUSD, cur),
			dayModelList(r.models),
		)
		if offset+i == m.daySel { // cursor row
			row = lipgloss.NewStyle().Reverse(true).Render(row)
		}
		b.WriteString(row)
		b.WriteByte('\n')
	}

	// Grand total over every day in range (not just the shown slice).
	var g usage.Totals
	for _, r := range rows {
		g.Input += r.totals.Input
		g.Output += r.totals.Output
		g.CacheRead += r.totals.CacheRead
		g.CacheCreate += r.totals.CacheCreate
		g.Total += r.totals.Total
		g.CostUSD += r.totals.CostUSD
	}
	note := ""
	if limit > 0 && len(rows) > limit {
		note = fmt.Sprintf("%d days", len(rows))
	}
	total := line("TOTAL", compact(g.Input), compact(g.Output), compact(g.CacheRead+g.CacheCreate),
		compact(g.Total), fmt.Sprintf("~%.2f %s", g.CostUSD, cur), note)
	b.WriteString(lipgloss.NewStyle().Foreground(colText).Bold(true).Render(total))
	return b.String()
}
