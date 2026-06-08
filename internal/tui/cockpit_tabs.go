package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tslc "github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

// airspeedTape renders a glass-cockpit airspeed tape: a horizontal scale with
// green/amber/red speed zones and a bright needle at the current value.
func airspeedTape(value, scaleMax float64, width int) string {
	if width < 12 {
		width = 12
	}
	if scaleMax <= 0 {
		scaleMax = 1
	}
	ratio := value / scaleMax
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	needle := int(ratio*float64(width-1) + 0.5)
	var b strings.Builder
	for i := 0; i < width; i++ {
		zr := float64(i) / float64(width-1)
		switch {
		case i == needle:
			b.WriteString(needleStyle.Render("▮"))
		case i < needle:
			st := zoneGreen
			switch {
			case zr >= 0.85:
				st = zoneRed
			case zr >= 0.6:
				st = zoneAmber
			}
			b.WriteString(st.Render("█"))
		default:
			b.WriteString(gaugeEmpty.Render("░"))
		}
	}
	return b.String()
}

// fmtScale formats an axis tick with precision that stays readable for small
// magnitudes (sub-1 token/sec) as well as large ones.
func fmtScale(f float64) string {
	switch {
	case f >= 10:
		return fmt.Sprintf("%.0f", f)
	case f >= 1:
		return fmt.Sprintf("%.1f", f)
	default:
		return fmt.Sprintf("%.2f", f)
	}
}

// axisLine places labels at the given rune positions within a width-wide line.
// Positions are applied left-to-right and a label is skipped if it would
// overlap the previous one, so the result is deterministic (map iteration order
// would otherwise make overlapping labels flicker between renders).
func axisLine(width int, labels map[int]string) string {
	if width < 1 {
		width = 1
	}
	buf := make([]rune, width)
	for i := range buf {
		buf[i] = ' '
	}
	positions := make([]int, 0, len(labels))
	for pos := range labels {
		positions = append(positions, pos)
	}
	sort.Ints(positions)
	lastEnd := -1
	for _, pos := range positions {
		if pos <= lastEnd {
			continue // would collide with the previous label
		}
		runes := []rune(labels[pos])
		for j, r := range runes {
			if p := pos + j; p >= 0 && p < width {
				buf[p] = r
			}
		}
		lastEnd = pos + len(runes) - 1
	}
	return string(buf)
}

// sparkRow renders a single-line sparkline for the given series.
func sparkRow(series []float64, width int, color lipgloss.Color) string {
	sl := sparkline.New(width, 1, sparkline.WithStyle(lipgloss.NewStyle().Foreground(color)))
	sl.PushAll(series)
	sl.Draw()
	return sl.View()
}

// kv renders a "LABEL   value" instrument line with a dim label.
func kv(label, value string, accent lipgloss.Color) string {
	return labelStyle.Render(fmt.Sprintf("%-10s", label)) +
		lipgloss.NewStyle().Foreground(accent).Bold(true).Render(value)
}

// agentsView is the per-engine instrument cluster: one panel per agent with
// its headline readouts, a share gauge, observed speed, and an activity trace.
func (m Model) agentsView(width int) string {
	prices := m.reportOptions.Pricing
	buckets := usage.GroupByWith(m.events, prices, func(e usage.Event) string { return e.Source })
	if len(buckets) == 0 {
		return labelStyle.Render("no data")
	}
	cur := m.currency()

	if m.compact {
		header := panel("◈ ENGINES · tokens", colCyan, width, m.enginesBar(m.events, prices, width))
		var b strings.Builder
		for _, bk := range buckets {
			bullet := lipgloss.NewStyle().Foreground(agentColor(bk.Key)).Render("●")
			fmt.Fprintf(&b, "%s %-8s %9s  %10s  %5.1f%%\n",
				bullet, truncate(bk.Key, 8), compact(bk.Totals.Total),
				fmt.Sprintf("%.2f %s", bk.Totals.CostUSD, cur), bk.Share*100)
		}
		list := panel("◈ AGENTS", colCyan, width, strings.TrimRight(b.String(), "\n"))
		return vstack(header, list)
	}

	gap := 1
	n := len(buckets)
	pw, stack := gridWidths(width, gap, n, 30)
	if stack {
		pw = width
	}

	panels := make([]string, 0, n)
	for _, b := range buckets {
		c := agentColor(b.Key)
		sub := filterSource(m.events, b.Key)
		tps := subsetSpeed(sub)
		sparkW := pw - 13 // box(6) + label(7)
		if sparkW < 6 {
			sparkW = 6
		}
		spark := sparkRow(dailySeriesFor(m.events, 14, b.Key), sparkW, c)
		gaugeW := pw - 19 // box(6) + label(7) + " 00.0%"(6)
		if gaugeW < 6 {
			gaugeW = 6
		}
		body := lipgloss.JoinVertical(lipgloss.Left,
			kv("TOKENS", compact(b.Totals.Total), c),
			kv("COST", fmt.Sprintf("%.2f %s", b.Totals.CostUSD, cur), colGreen),
			kv("EVENTS", compact(int64(b.Totals.Events)), colText),
			kv("OUTPUT", compact(b.Totals.Output), colText),
			kv("SPEED", fmt.Sprintf("%.1f t/s", tps), colAmber),
			"",
			labelStyle.Render(fmt.Sprintf("%-7s", "SHARE"))+gaugeColored(b.Share, gaugeW, c)+fmt.Sprintf(" %4.1f%%", b.Share*100),
			labelStyle.Render(fmt.Sprintf("%-7s", "14d"))+spark,
		)
		panels = append(panels, panel("◈ "+strings.ToUpper(b.Key), c, pw, body))
	}

	row := arrangePanels(stack, gap, panels...)
	header := panel("◈ ENGINES · tokens", colCyan, width, m.enginesBar(m.events, prices, width))
	return lipgloss.JoinVertical(lipgloss.Left, header, row)
}

// modelsView is the model instrument table: a gauge, cost, and share per model.
func (m Model) modelsView(width int) string {
	limit := 10
	if m.compact {
		limit = 5
	}
	return panel("◈ MODELS · load", colCyan, width, m.modelsBody(width, limit))
}

// modelsBody renders the per-model load/cost/share table (limit<=0 = all).
func (m Model) modelsBody(width, limit int) string {
	prices := m.reportOptions.Pricing
	buckets := usage.GroupByWith(m.events, prices, func(e usage.Event) string { return e.Model })
	if len(buckets) == 0 {
		return labelStyle.Render("no data")
	}
	if limit > 0 && len(buckets) > limit {
		buckets = buckets[:limit]
	}
	cur := m.currency()
	inner := width - 6
	maxTok := buckets[0].Totals.Total
	if maxTok < 1 {
		maxTok = 1
	}
	barW := inner - 50 // name(18)+gap+bar+gap+tokens(9)+gap+cost(11)+gap+share(6)
	if barW < 8 {
		barW = 8
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", labelStyle.Render(fmt.Sprintf("%-18s %-*s %9s %11s %6s", "MODEL", barW, "LOAD", "TOKENS", "COST", "SHARE")))
	for _, bk := range buckets {
		ratio := float64(bk.Totals.Total) / float64(maxTok)
		name := truncate(bk.Key, 18)
		fmt.Fprintf(&b, "%-18s %s %9s %11s %5.1f%%\n",
			name, gauge(ratio, barW), compact(bk.Totals.Total),
			fmt.Sprintf("%.2f %s", bk.Totals.CostUSD, cur), bk.Share*100)
	}
	return strings.TrimRight(b.String(), "\n")
}

// trendsView is the navigation display: stacked token and cost time-series with
// summary readouts (total, daily average, peak).
func (m Model) trendsView(width int) string {
	days := 30
	prices := m.reportOptions.Pricing
	cur := m.currency()

	tokPts := dailySeries(m.events, days)
	costPts := dailyCostSeries(m.events, days, prices)

	var totalTok, peakTok float64
	for _, p := range tokPts {
		totalTok += p.Value
		if p.Value > peakTok {
			peakTok = p.Value
		}
	}
	var totalCost, peakCost float64
	for _, p := range costPts {
		totalCost += p.Value
		if p.Value > peakCost {
			peakCost = p.Value
		}
	}

	readouts := lipgloss.JoinHorizontal(lipgloss.Top, spread([]string{
		readout("WINDOW", fmt.Sprintf("%dd", days), colDim),
		readout("TOKENS", compact(int64(totalTok)), colCyan),
		readout("TOK/DAY", compact(int64(totalTok/float64(days))), colText),
		readout("PEAK TOK", compact(int64(peakTok)), colGreen),
		readout("COST", fmt.Sprintf("%.2f %s", totalCost, cur), colGreen),
		readout("PEAK COST", fmt.Sprintf("%.2f", peakCost), colAmber),
	}, "   ")...)

	nav := heroPanel("✈ NAV · trends", colCyan, width, readouts)
	tokChart := panel("◈ TOKENS · "+fmt.Sprintf("%dd", days), colCyan, width,
		seriesChart(tokPts, width, 9, colGreen))
	if m.compact {
		return vstack(nav, tokChart)
	}
	costChart := panel("◈ COST · "+fmt.Sprintf("%dd", days), colAmber, width,
		seriesChart(costPts, width, 9, colAmber))
	return vstack(nav, tokChart, costChart)
}

// speedView is the airspeed panel: observed output tokens/sec per agent/model,
// rendered as gauges relative to the fastest lane.
func (m Model) speedView(width int) string {
	rows := speedRows(m.events)
	if len(rows) == 0 {
		return labelStyle.Render("no data")
	}
	limit := 16
	if m.compact {
		limit = 5
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	top := rows[0]
	maxTPS := top.tps
	if maxTPS <= 0 {
		maxTPS = 1
	}
	inner := width - 6

	// Hero: a glass-cockpit airspeed tape driven by the fastest lane, scaled
	// with headroom so the needle reads in the normal band, not the redline.
	scaleMax := maxTPS * 1.25
	tape := airspeedTape(top.tps, scaleMax, inner)
	rmax := fmtScale(scaleMax)
	mid := fmtScale(scaleMax / 2)
	scale := labelStyle.Render(axisLine(inner, map[int]string{
		0:                              "0",
		(inner - len([]rune(mid))) / 2: mid,
		inner - len([]rune(rmax)):      rmax,
	}))
	readouts := lipgloss.JoinHorizontal(lipgloss.Top, spread([]string{
		readout("FASTEST", top.key, colGreen),
		readout("OUT t/s", fmt.Sprintf("%.1f", top.tps), colCyan),
		readout("LANES", compact(int64(len(rows))), colDim),
	}, "   ")...)
	hero := heroPanel("✈ AIRSPEED · output t/s", colCyan, width,
		lipgloss.JoinVertical(lipgloss.Left, readouts, "", scale, tape))

	return lipgloss.JoinVertical(lipgloss.Left, hero,
		panel("◈ OUTPUT SPEED · all lanes", colCyan, width, speedLanes(rows, maxTPS, width)))
}

// speedLanes renders the per-lane relative-speed table.
func speedLanes(rows []speedRow, maxTPS float64, width int) string {
	if maxTPS <= 0 {
		maxTPS = 1
	}
	barW := width - 6 - 44 // label(24)+gap+bar+gap+tps(10)+gap+events(6)
	if barW < 8 {
		barW = 8
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", labelStyle.Render(fmt.Sprintf("%-24s %-*s %10s %6s", "AGENT / MODEL", barW, "RELATIVE", "OUT t/s", "EVENTS")))
	for _, r := range rows {
		label := truncate(r.key, 24)
		fmt.Fprintf(&b, "%-24s %s %8.1f   %6d\n",
			label, gauge(r.tps/maxTPS, barW), r.tps, r.events)
	}
	return strings.TrimRight(b.String(), "\n")
}

// --- shared helpers ---

func (m Model) currency() string {
	if m.reportOptions.Currency == "" {
		return "USD"
	}
	return m.reportOptions.Currency
}

func filterSource(events []usage.Event, source string) []usage.Event {
	out := make([]usage.Event, 0, len(events))
	for _, e := range events {
		if e.Source == source {
			out = append(out, e)
		}
	}
	return out
}

type speedRow struct {
	key    string
	tps    float64
	tokens int64
	events int
}

func speedRows(events []usage.Event) []speedRow {
	type acc struct {
		first, last time.Time
		tokens      int64
		events      int
	}
	byKey := map[string]*acc{}
	for _, e := range events {
		key := e.Source
		if e.Model != "" {
			key += " / " + e.Model
		}
		a := byKey[key]
		if a == nil {
			a = &acc{first: e.Timestamp, last: e.Timestamp}
			byKey[key] = a
		}
		if e.Timestamp.Before(a.first) {
			a.first = e.Timestamp
		}
		if e.Timestamp.After(a.last) {
			a.last = e.Timestamp
		}
		a.tokens += e.Output + e.Reasoning
		a.events++
	}
	rows := make([]speedRow, 0, len(byKey))
	for key, a := range byKey {
		var tps float64
		if secs := a.last.Sub(a.first).Seconds(); secs > 0 {
			tps = float64(a.tokens) / secs
		}
		rows = append(rows, speedRow{key: key, tps: tps, tokens: a.tokens, events: a.events})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].tps > rows[j].tps })
	return rows
}

func subsetSpeed(events []usage.Event) float64 {
	if len(events) == 0 {
		return 0
	}
	first, last := events[0].Timestamp, events[0].Timestamp
	var tokens int64
	for _, e := range events {
		if e.Timestamp.Before(first) {
			first = e.Timestamp
		}
		if e.Timestamp.After(last) {
			last = e.Timestamp
		}
		tokens += e.Output + e.Reasoning
	}
	secs := last.Sub(first).Seconds()
	if secs <= 0 {
		return 0
	}
	return float64(tokens) / secs
}

func dailyCostSeries(events []usage.Event, days int, prices usage.PriceBook) []tslc.TimePoint {
	start := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)
	buckets := make([]float64, days)
	for _, e := range events {
		idx := int(e.Timestamp.Truncate(24*time.Hour).Sub(start) / (24 * time.Hour))
		if idx >= 0 && idx < days {
			buckets[idx] += usage.EstimateCostWith(e, prices)
		}
	}
	pts := make([]tslc.TimePoint, days)
	for i := range buckets {
		pts[i] = tslc.TimePoint{Time: start.AddDate(0, 0, i), Value: buckets[i]}
	}
	return pts
}

// seriesChart renders a braille time-series line for the given points.
func seriesChart(pts []tslc.TimePoint, width, height int, color lipgloss.Color) string {
	inner := width - 6
	if inner < 20 {
		inner = 20
	}
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
	tc := tslc.New(inner, height,
		tslc.WithTimeRange(start, end),
		tslc.WithYRange(0, maxY),
		tslc.WithAxesStyles(labelStyle, labelStyle),
		tslc.WithStyle(lipgloss.NewStyle().Foreground(color)),
	)
	tc.SetViewTimeAndYRange(start, end, 0, maxY)
	for _, p := range pts {
		tc.Push(p)
	}
	tc.DrawBraille()
	return tc.View()
}
