package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

// usageDetailBody renders an everything-we-have breakdown for a set of events:
// headline readouts (two rows), a per-engine split, and a per-model table. Used
// by the Daily and Blocks drill-down popups.
func (m Model) usageDetailBody(events []usage.Event) string {
	prices := m.reportOptions.Pricing
	cur := m.currency()
	t := usage.SummarizeWith(events, prices)

	io := 0.0
	if t.Output > 0 {
		io = float64(t.Input) / float64(t.Output)
	}
	hit := 0.0
	if t.Input+t.CacheRead > 0 {
		hit = float64(t.CacheRead) / float64(t.Input+t.CacheRead) * 100
	}
	agents := usage.GroupByWith(events, prices, func(e usage.Event) string { return e.Source })
	models := usage.GroupByWith(events, prices, func(e usage.Event) string { return e.Model })

	row1 := lipgloss.JoinHorizontal(lipgloss.Top, spread([]string{
		readout("EVENTS", compact(int64(t.Events)), colCyan),
		readout("TOKENS", compact(t.Total), colCyan),
		readout("COST", fmt.Sprintf("~%.2f %s", t.CostUSD, cur), colGreen),
		readout("IN:OUT", fmt.Sprintf("%.1f:1", io), colText),
		readout("CACHE HIT", fmt.Sprintf("%.0f%%", hit), colText),
		readout("MODELS", compact(int64(len(models))), colGreen),
	}, "   ")...)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, spread([]string{
		readout("INPUT", compact(t.Input), colText),
		readout("OUTPUT", compact(t.Output), colText),
		readout("CACHE R", compact(t.CacheRead), colText),
		readout("CACHE W", compact(t.CacheCreate), colText),
		readout("REASONING", compact(t.Reasoning), colText),
	}, "   ")...)

	var ab strings.Builder
	ab.WriteString(labelStyle.Render("BY ENGINE"))
	ab.WriteByte('\n')
	for _, a := range agents {
		ab.WriteString(kv(strings.ToUpper(a.Key),
			fmt.Sprintf("%-9s  ~%.2f %s  %4.1f%%", compact(a.Totals.Total), a.Totals.CostUSD, cur, a.Share*100),
			agentColor(a.Key)))
		ab.WriteByte('\n')
	}

	const nameW, numW, totW, costW = 22, 9, 11, 12
	line := func(name, in, out, cache, tot, cost, share string) string {
		return fmt.Sprintf("%-*s %*s %*s %*s %*s %*s %6s",
			nameW, truncate(name, nameW), numW, in, numW, out, numW, cache, totW, tot, costW, cost, share)
	}
	var mb strings.Builder
	mb.WriteString(labelStyle.Render("BY MODEL"))
	mb.WriteByte('\n')
	mb.WriteString(labelStyle.Render(line("MODEL", "INPUT", "OUTPUT", "CACHE", "TOTAL", "COST", "SHARE")))
	mb.WriteByte('\n')
	for _, bk := range models {
		x := bk.Totals
		mb.WriteString(line(shortModel(bk.Key), compact(x.Input), compact(x.Output),
			compact(x.CacheRead+x.CacheCreate), compact(x.Total),
			fmt.Sprintf("~%.2f %s", x.CostUSD, cur), fmt.Sprintf("%.0f%%", bk.Share*100)))
		mb.WriteByte('\n')
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		row1, row2, "",
		ab.String(),
		strings.TrimRight(mb.String(), "\n"),
	)
}
