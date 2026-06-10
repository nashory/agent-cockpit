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
	start  time.Time
	end    time.Time
	totals usage.Totals
	models map[string]struct{}
}

// dailyLedger aggregates events into one row per calendar day, newest first,
// ccusage-style: per-day input/output/cache/total tokens, cost, and the set of
// models touched that day.
func dailyLedger(events []usage.Event, prices usage.PriceBook) []dayRow {
	return periodLedger(events, prices, 0)
}

func periodLabel(mode int) string {
	switch mode {
	case 1:
		return "weekly"
	case 2:
		return "monthly"
	default:
		return "daily"
	}
}

func periodBounds(t time.Time, mode int) (string, time.Time, time.Time) {
	switch mode {
	case 1:
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, 1-wd)
		y, w := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w), start, start.AddDate(0, 0, 7)
	case 2:
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		return t.Format("2006-01"), start, start.AddDate(0, 1, 0)
	default:
		start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		return t.Format("2006-01-02"), start, start.AddDate(0, 0, 1)
	}
}

func periodLedger(events []usage.Event, prices usage.PriceBook, mode int) []dayRow {
	byDay := map[string]*dayRow{}
	for _, e := range events {
		if e.Timestamp.IsZero() {
			continue
		}
		k, start, end := periodBounds(e.Timestamp, mode)
		r := byDay[k]
		if r == nil {
			r = &dayRow{date: k, start: start, end: end, models: map[string]struct{}{}}
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
	sort.Slice(rows, func(i, j int) bool { return rows[i].start.After(rows[j].start) }) // newest first
	return rows
}

// sortedLedger returns the per-day rows ordered by the active sort mode: date
// (newest first, the default), tokens, or cost (both descending).
func (m Model) sortedLedger() []dayRow {
	rows := periodLedger(m.events, m.reportOptions.Pricing, m.periodMode)
	switch m.sortMode {
	case 1:
		sort.Slice(rows, func(i, j int) bool { return rows[i].totals.Total > rows[j].totals.Total })
	case 2:
		sort.Slice(rows, func(i, j int) bool { return rows[i].totals.CostUSD > rows[j].totals.CostUSD })
	}
	return rows
}

// sortLabel names the active table sort for a title.
func sortLabel(mode int) string {
	switch mode {
	case 1:
		return "tokens"
	case 2:
		return "cost"
	}
	return "date"
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
	title := fmt.Sprintf("◈ LEDGER · %s · last %s · row %d/%d · sort %s · p period · ↑↓ enter · s sort",
		periodLabel(m.periodMode), span, m.daySel+1, m.tableTotal(), sortLabel(m.sortMode))
	return panel(title, colCyan, width, body)
}

// dayDetail renders the per-model breakdown for the selected day (enter on the
// Daily tab), the drill-down ccusage shows on its daily rows.
func (m Model) dayDetail(width int) string {
	rows := m.sortedLedger()
	if m.daySel < 0 || m.daySel >= len(rows) {
		return panel("◈ DAILY", colCyan, width, labelStyle.Render("no data"))
	}
	row := rows[m.daySel]

	var dayEvents []usage.Event
	for _, e := range m.events {
		if !e.Timestamp.IsZero() && !e.Timestamp.Before(row.start) && e.Timestamp.Before(row.end) {
			dayEvents = append(dayEvents, e)
		}
	}
	label := row.date
	if m.periodMode == 0 {
		label = row.start.Format("2006-01-02 Mon")
	}
	header := fmt.Sprintf("⤢ LEDGER · %s   ·   esc back", label)
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
	rows := m.sortedLedger()
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
	b.WriteString(labelStyle.Render(line("PERIOD", "INPUT", "OUTPUT", "CACHE", "TOTAL", "COST", "MODELS")))
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
		note = fmt.Sprintf("%d %s rows", len(rows), periodLabel(m.periodMode))
	}
	total := line("TOTAL", compact(g.Input), compact(g.Output), compact(g.CacheRead+g.CacheCreate),
		compact(g.Total), fmt.Sprintf("~%.2f %s", g.CostUSD, cur), note)
	b.WriteString(lipgloss.NewStyle().Foreground(colText).Bold(true).Render(total))
	return b.String()
}
