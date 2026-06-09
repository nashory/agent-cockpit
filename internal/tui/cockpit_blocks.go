package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

// fmtDur renders a duration as "2h05m" or "45m" for the block readouts.
func fmtDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// blocksView is the billing-window tab: an ACTIVE WINDOW hero (how much of the
// current 5-hour window is spent, with a burn-rate projection) over a table of
// recent windows, mirroring ccusage's blocks report.
func (m Model) blocksView(width int) string {
	bl := usage.SessionBlocks(m.events, m.reportOptions.Pricing, usage.DefaultBlockWindow)
	if m.blkPopup {
		return m.blockDetail(width, bl)
	}
	hero := heroPanel("✈ ACTIVE WINDOW · 5h", colCyan, width, m.activeBlockBody(bl))

	vis := m.tableVisible()
	title := "◈ BLOCKS · 5h windows  · ↑↓ select · enter window"
	table := panel(title, colCyan, width, m.blocksTable(bl, width, m.scroll, vis))
	return vstack(hero, table)
}

// blockDetail renders the per-model breakdown for the selected 5-hour window
// (enter on the Blocks tab).
func (m Model) blockDetail(width int, bl []usage.Block) string {
	// blkSel indexes the newest-first display order.
	idx := len(bl) - 1 - m.blkSel
	if idx < 0 || idx >= len(bl) {
		return panel("◈ BLOCKS", colCyan, width, labelStyle.Render("no data"))
	}
	blk := bl[idx]
	prices := m.reportOptions.Pricing
	cur := m.currency()

	var winEvents []usage.Event
	for _, e := range m.events {
		if e.Timestamp.IsZero() {
			continue
		}
		if !e.Timestamp.Before(blk.Start) && e.Timestamp.Before(blk.End) {
			winEvents = append(winEvents, e)
		}
	}
	buckets := usage.GroupByWith(winEvents, prices, func(e usage.Event) string { return e.Model })

	const nameW, numW, totW, costW = 22, 9, 11, 12
	line := func(name, in, out, cache, tot, cost string) string {
		return fmt.Sprintf("%-*s %*s %*s %*s %*s %*s",
			nameW, truncate(name, nameW), numW, in, numW, out, numW, cache, totW, tot, costW, cost)
	}
	var b strings.Builder
	b.WriteString(labelStyle.Render(line("MODEL", "INPUT", "OUTPUT", "CACHE", "TOTAL", "COST")))
	b.WriteByte('\n')
	for _, bk := range buckets {
		t := bk.Totals
		b.WriteString(line(shortModel(bk.Key), compact(t.Input), compact(t.Output),
			compact(t.CacheRead+t.CacheCreate), compact(t.Total),
			fmt.Sprintf("~%.2f %s", t.CostUSD, cur)))
		b.WriteByte('\n')
	}
	t := blk.Totals
	b.WriteString(lipgloss.NewStyle().Foreground(colText).Bold(true).Render(
		line("TOTAL", compact(t.Input), compact(t.Output), compact(t.CacheRead+t.CacheCreate),
			compact(t.Total), fmt.Sprintf("~%.2f %s", t.CostUSD, cur))))

	header := fmt.Sprintf("⤢ WINDOW · %s-%s   ·   esc back",
		blk.Start.Format("01-02 15:04"), blk.End.Format("15:04"))
	return heroPanel(header, colCyan, width, b.String())
}

// activeBlockBody renders the readouts for the window containing "now": elapsed,
// remaining, tokens, cost, burn rate, and the projected cost if the current
// burn holds to the end of the window.
func (m Model) activeBlockBody(bl []usage.Block) string {
	cur := m.currency()
	var active *usage.Block
	for i := range bl {
		if bl[i].Active {
			active = &bl[i]
			break
		}
	}
	if active == nil {
		return labelStyle.Render("no active window · start working and the live 5h burn shows up here")
	}
	now := time.Now()
	elapsed := now.Sub(active.Start)
	if elapsed <= 0 {
		elapsed = time.Minute
	}
	left := active.End.Sub(now)
	t := active.Totals
	hrs := elapsed.Hours()
	burn := float64(t.Total) / hrs
	proj := t.CostUSD * usage.DefaultBlockWindow.Hours() / hrs

	cells := []string{
		readout("WINDOW", active.Start.Format("15:04")+"-"+active.End.Format("15:04"), colCyan),
		readout("LEFT", fmtDur(left), colAmber),
		readout("TOKENS", compact(t.Total), colGreen),
		readout("COST", fmt.Sprintf("~%.2f %s", t.CostUSD, cur), colGreen),
		readout("BURN", compact(int64(burn))+"/h", colText),
		readout("PROJ", fmt.Sprintf("~%.2f %s", proj, cur), colAmber),
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, spread(cells, "   ")...)
}

// blocksTable lists windows newest first: when it started, how long was active,
// tokens, cost, and models, with a marker on the live window.
func (m Model) blocksTable(bl []usage.Block, width, offset, limit int) string {
	if len(bl) == 0 {
		return labelStyle.Render("no data")
	}
	cur := m.currency()
	inner := width - 6
	if inner < 40 {
		inner = 40
	}
	const markW, startW, durW, numW, costW = 2, 14, 8, 10, 12
	modelsW := inner - markW - startW - durW - numW - costW - 5
	if modelsW < 8 {
		modelsW = 8
	}
	line := func(mark, start, dur, tok, cost, models string) string {
		return fmt.Sprintf("%-*s %-*s %*s %*s %*s %-*s",
			markW, mark, startW, start, durW, dur, numW, tok, costW, cost,
			modelsW, truncate(models, modelsW))
	}

	var b strings.Builder
	b.WriteString(labelStyle.Render(line("", "STARTED", "ACTIVE", "TOKENS", "COST", "MODELS")))
	b.WriteByte('\n')

	// Newest first.
	rows := make([]usage.Block, len(bl))
	for i, x := range bl {
		rows[len(bl)-1-i] = x
	}
	if offset > 0 && offset < len(rows) {
		rows = rows[offset:]
	} else if offset >= len(rows) {
		rows = nil
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	for i, blk := range rows {
		mark := " "
		accent := colText
		if blk.Active {
			mark = "●"
			accent = colGreen
		}
		row := line(mark,
			blk.Start.Format("01-02 15:04"),
			fmtDur(blk.LastActivity.Sub(blk.Start)),
			compact(blk.Totals.Total),
			fmt.Sprintf("~%.2f %s", blk.Totals.CostUSD, cur),
			dayModelList(blk.Models),
		)
		st := lipgloss.NewStyle().Foreground(accent)
		if offset+i == m.blkSel { // cursor row
			st = st.Reverse(true)
		}
		b.WriteString(st.Render(row))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
