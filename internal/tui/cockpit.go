package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/barchart"
	tslc "github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

// overview renders the glass-cockpit dashboard: bordered instrument panels
// laid out in a responsive grid. width is the available content width.
func (m Model) overview(width int) string {
	if width < 40 {
		width = 40
	}
	events := m.events
	prices := m.reportOptions.Pricing
	totals := usage.SummarizeWith(events, prices)

	primary := m.primaryStrip(totals, width)

	if m.compact {
		// Light: headline readouts + the two instruments that matter most.
		engines := panel("◈ ENGINES · "+m.dataSpanLabel(), colCyan, width, m.enginesBar(events, prices, width))
		trend := panel("◈ TREND · "+fmt.Sprintf("%dd", m.windowDays)+" tokens", colCyan, width, m.trendChart(events, width))
		return vstack(primary, engines, trend)
	}

	gap := 1
	cw, stack := gridWidths(width, gap, 2, 40)
	lw, rw := cw, cw
	if stack {
		lw, rw = width, width
	}

	annun := m.annunciator(events, totals, width)

	// Two rows of paired instruments; panels in a row share a box height so the
	// grid stays even. Narrow terminals stack everything vertically.
	span := m.dataSpanLabel()
	row1 := panelsRow(stack, gap,
		panelSpec{"◈ ENGINES · " + span, colCyan, lw, m.enginesBar(events, prices, lw)},
		panelSpec{"◈ TREND · " + fmt.Sprintf("%dd", m.windowDays) + " tokens", colGreen, rw, m.trendChart(events, rw)},
	)
	row2 := panelsRow(stack, gap,
		panelSpec{"◈ MODELS · " + span, colCyan, lw, m.modelsBar(events, prices, lw)},
		panelSpec{"◈ ACTIVITY · 14d", colCyan, rw, m.agentSparks(events, rw)},
	)
	mid := lipgloss.JoinVertical(lipgloss.Left, row1, row2)
	return lipgloss.JoinVertical(lipgloss.Left, primary, mid, annun)
}

// primaryStrip is the PFD-style top row: the headline readouts.
// dataSpanLabel describes the calendar window the loaded events actually cover
// (e.g. "30d"), so all-time aggregates like ENGINES share and MODELS load state
// their period instead of leaving the reader guessing. It reflects any active
// --since/--days filter since those shrink the loaded set.
func (m Model) dataSpanLabel() string {
	d := m.ins.SpanDays
	if d < 1 {
		d = 1
	}
	return fmt.Sprintf("%dd", d)
}

func (m Model) primaryStrip(t usage.Totals, width int) string {
	cur := m.reportOptions.Currency
	if cur == "" {
		cur = "USD"
	}
	cells := []string{
		readout("TOKENS", compact(t.Total), colCyan),
		readout("COST", fmt.Sprintf("~%.2f %s", t.CostUSD, cur), colGreen),
		readout("EVENTS", compact(int64(t.Events)), colText),
		readout("CACHE", compact(t.CacheRead+t.CacheCreate), colDim),
	}
	clock := time.Now().Format("15:04:05")
	cells = append(cells, readout("CLOCK", clock, colAmber))

	joined := lipgloss.JoinHorizontal(lipgloss.Top, spread(cells, "   ")...)
	// These are all-time totals; state the span so COST etc. are not ambiguous.
	return heroPanel("✈ AGENT COCKPIT · "+m.dataSpanLabel(), colCyan, width, joined)
}

func readout(label, value string, accent lipgloss.Color) string {
	l := labelStyle.Render(label)
	v := lipgloss.NewStyle().Foreground(accent).Bold(true).Render(value)
	return lipgloss.JoinVertical(lipgloss.Left, l, v)
}

func spread(cells []string, sep string) []string {
	out := make([]string, 0, len(cells)*2)
	for i, c := range cells {
		if i > 0 {
			out = append(out, lipgloss.NewStyle().Foreground(colDim).Render(sep))
		}
		out = append(out, c)
	}
	return out
}

// enginesBar draws a horizontal barchart of token usage per agent.
func (m Model) enginesBar(events []usage.Event, prices usage.PriceBook, width int) string {
	buckets := usage.GroupByWith(events, prices, func(e usage.Event) string { return e.Source })
	if len(buckets) == 0 {
		return labelStyle.Render("no data")
	}
	inner := width - 6
	if inner < 10 {
		inner = 10
	}
	h := len(buckets)
	if h < 1 {
		h = 1
	}
	bc := barchart.New(inner, h,
		barchart.WithHorizontalBars(),
		barchart.WithNoAxis(),
		barchart.WithBarWidth(1),
		barchart.WithBarGap(0),
	)
	data := make([]barchart.BarData, 0, len(buckets))
	for _, b := range buckets {
		data = append(data, barchart.BarData{
			Label: b.Key,
			Values: []barchart.BarValue{{
				Name:  b.Key,
				Value: float64(b.Totals.Total),
				Style: lipgloss.NewStyle().Foreground(agentColor(b.Key)),
			}},
		})
	}
	bc.PushAll(data)
	bc.Draw()

	// Legend with share% under the bars.
	var legend strings.Builder
	for _, b := range buckets {
		c := agentColor(b.Key)
		fmt.Fprintf(&legend, "%s %-7s %5.1f%%  %s\n",
			lipgloss.NewStyle().Foreground(c).Render("●"),
			b.Key, b.Share*100, compact(b.Totals.Total))
	}
	return lipgloss.JoinVertical(lipgloss.Left, bc.View(), strings.TrimRight(legend.String(), "\n"))
}

// modelsBar draws a horizontal barchart of the top models by tokens.
func (m Model) modelsBar(events []usage.Event, prices usage.PriceBook, width int) string {
	buckets := usage.GroupByWith(events, prices, func(e usage.Event) string { return e.Model })
	if len(buckets) == 0 {
		return labelStyle.Render("no data")
	}
	if len(buckets) > 5 {
		buckets = buckets[:5]
	}
	inner := width - 6
	if inner < 10 {
		inner = 10
	}
	var b strings.Builder
	maxTok := buckets[0].Totals.Total
	if maxTok < 1 {
		maxTok = 1
	}
	barW := inner - 24 // name(14) + 2 spaces + value(8)
	if barW < 6 {
		barW = 6
	}
	for _, bk := range buckets {
		ratio := float64(bk.Totals.Total) / float64(maxTok)
		name := truncate(bk.Key, 14)
		fmt.Fprintf(&b, "%-14s %s %8s\n", name, gauge(ratio, barW), compact(bk.Totals.Total))
	}
	return strings.TrimRight(b.String(), "\n")
}

// trendChart draws a braille time-series line of daily total tokens.
func (m Model) trendChart(events []usage.Event, width int) string {
	days := m.windowDays
	inner := width - 6
	if inner < 20 {
		inner = 20
	}
	h := 8
	pts := dailySeries(events, days)
	if len(pts) == 0 {
		return labelStyle.Render("no data")
	}
	var maxY float64
	for _, p := range pts {
		if p.Value > maxY {
			maxY = p.Value
		}
	}
	if maxY <= 0 {
		maxY = 1
	}
	start := pts[0].Time
	end := pts[len(pts)-1].Time

	tc := tslc.New(inner, h,
		tslc.WithTimeRange(start, end),
		tslc.WithYRange(0, maxY),
		tslc.WithAxesStyles(labelStyle, labelStyle),
		tslc.WithStyle(lipgloss.NewStyle().Foreground(colGreen)),
	)
	tc.SetViewTimeAndYRange(start, end, 0, maxY)
	for _, p := range pts {
		tc.Push(p)
	}
	tc.DrawBraille()
	return tc.View()
}

// agentSparks renders a stacked sparkline per agent: a quick activity trace.
func (m Model) agentSparks(events []usage.Event, width int) string {
	inner := width - 6
	if inner < 12 {
		inner = 12
	}
	sparkW := inner - 10
	if sparkW < 6 {
		sparkW = 6
	}
	order := []string{"claude", "codex", "gemini"}
	var b strings.Builder
	for _, name := range order {
		series := dailySeriesFor(events, 14, name)
		spark := sparkRow(series, sparkW, agentColor(name))
		label := lipgloss.NewStyle().Foreground(agentColor(name)).Render(fmt.Sprintf("%-7s", name))
		fmt.Fprintf(&b, "%s %s\n", label, spark)
	}
	return strings.TrimRight(b.String(), "\n")
}

// annunciator is the bottom caution/warning lamp strip.
func (m Model) annunciator(events []usage.Event, t usage.Totals, width int) string {
	lamps := []string{}

	// LIVE / SNAPSHOT mode.
	if m.refreshInterval > 0 {
		lamps = append(lamps, okStyle.Render("● LIVE "+m.refreshInterval.String()))
	} else {
		lamps = append(lamps, darkLamp.Render("○ SNAPSHOT"))
	}

	// OPUS HEAVY: expensive-model token share.
	var opus int64
	for _, e := range events {
		if strings.Contains(strings.ToLower(e.Model), "opus") {
			opus += e.TotalTokens()
		}
	}
	if t.Total > 0 && float64(opus)/float64(t.Total) > 0.4 {
		lamps = append(lamps, litStyle.Render(" ◆ OPUS HEAVY "))
	} else {
		lamps = append(lamps, darkLamp.Render("◇ opus"))
	}

	// HIGH BURN: latest day well above the daily average.
	series := dailySeries(events, 14)
	if hot := burnHot(series); hot {
		lamps = append(lamps, alarmStyle.Render(" ⚠ HIGH BURN "))
	} else {
		lamps = append(lamps, darkLamp.Render("△ burn"))
	}

	// STALE: newest event older than an hour.
	if stale(events) {
		lamps = append(lamps, warnStyle.Render("⧖ STALE"))
	} else {
		lamps = append(lamps, darkLamp.Render("⧗ fresh"))
	}

	body := strings.Join(lamps, lipgloss.NewStyle().Foreground(colDim).Render("   "))
	return heroPanel("◈ ANNUNCIATOR", colAmber, width, body)
}

// --- data helpers ---

func dailySeries(events []usage.Event, days int) []tslc.TimePoint {
	start := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)
	buckets := make([]float64, days)
	for _, e := range events {
		idx := int(e.Timestamp.Truncate(24*time.Hour).Sub(start) / (24 * time.Hour))
		if idx >= 0 && idx < days {
			buckets[idx] += float64(e.TotalTokens())
		}
	}
	pts := make([]tslc.TimePoint, days)
	for i := range buckets {
		pts[i] = tslc.TimePoint{Time: start.AddDate(0, 0, i), Value: buckets[i]}
	}
	return pts
}

func dailySeriesFor(events []usage.Event, days int, source string) []float64 {
	start := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)
	buckets := make([]float64, days)
	for _, e := range events {
		if e.Source != source {
			continue
		}
		idx := int(e.Timestamp.Truncate(24*time.Hour).Sub(start) / (24 * time.Hour))
		if idx >= 0 && idx < days {
			buckets[idx] += float64(e.TotalTokens())
		}
	}
	return buckets
}

func burnHot(series []tslc.TimePoint) bool {
	if len(series) < 3 {
		return false
	}
	var sum float64
	for _, p := range series[:len(series)-1] {
		sum += p.Value
	}
	avg := sum / float64(len(series)-1)
	last := series[len(series)-1].Value
	return avg > 0 && last > avg*1.5
}

func stale(events []usage.Event) bool {
	var newest time.Time
	for _, e := range events {
		if e.Timestamp.After(newest) {
			newest = e.Timestamp
		}
	}
	if newest.IsZero() {
		return false
	}
	return time.Since(newest) > time.Hour
}

func compact(n int64) string {
	f := float64(n)
	switch {
	case f >= 1e12:
		return fmt.Sprintf("%.2fT", f/1e12)
	case f >= 1e9:
		return fmt.Sprintf("%.2fB", f/1e9)
	case f >= 1e6:
		return fmt.Sprintf("%.1fM", f/1e6)
	case f >= 1e3:
		return fmt.Sprintf("%.1fK", f/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}
