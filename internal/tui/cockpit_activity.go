package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

var weekdayNames = [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func hourLabel(h int) string {
	if h < 0 || h > 23 {
		return "—"
	}
	return fmt.Sprintf("%02d:00", h)
}

func weekdayLabel(d int) string {
	if d < 0 || d > 6 {
		return "—"
	}
	return weekdayNames[d]
}

// heatRamp goes cold (idle) to hot (busy), glass-cockpit style.
var heatRamp = []lipgloss.Color{
	lipgloss.Color("236"), lipgloss.Color("238"), lipgloss.Color("24"),
	lipgloss.Color("30"), lipgloss.Color("37"), lipgloss.Color("44"),
	lipgloss.Color("48"), lipgloss.Color("148"), lipgloss.Color("214"),
	lipgloss.Color("208"), lipgloss.Color("196"),
}

func heatStyle(ratio float64) lipgloss.Style {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	idx := int(ratio*float64(len(heatRamp)-1) + 0.5)
	if idx >= len(heatRamp) {
		idx = len(heatRamp) - 1
	}
	return lipgloss.NewStyle().Foreground(heatRamp[idx])
}

// heatCells renders one colored block group per value, cellW wide each.
func heatCells(values []int64, cellW int) string {
	var max int64 = 1
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	block := strings.Repeat("█", cellW)
	for _, v := range values {
		b.WriteString(heatStyle(float64(v) / float64(max)).Render(block))
	}
	return b.String()
}

// activityView is the temporal-rhythm display: when work happens (hour of day,
// day of week) and where it lands (top projects).
func (m Model) activityView(width int) string {
	ins := m.ins
	cur := m.currency()

	hero := heroPanel("✈ ACTIVITY", colCyan, width,
		lipgloss.JoinHorizontal(lipgloss.Top, spread([]string{
			readout("PEAK HOUR", hourLabel(ins.BusiestHour), colCyan),
			readout("BUSIEST DAY", weekdayLabel(ins.BusiestWeekday), colGreen),
			readout("ACTIVE DAYS", fmt.Sprintf("%d / %d", ins.ActiveDays, ins.SpanDays), colText),
			readout("PROJECTS", compact(int64(ins.Projects)), colAmber),
		}, "   ")...))

	hourPanel := panel("◈ HOUR OF DAY · tokens", colCyan, width, m.hourStrip(width))

	if m.compact {
		return vstack(hero, hourPanel)
	}

	// DAY OF WEEK bars + TOP PROJECTS, side by side (stacked when narrow).
	gap := 1
	cw, stack := gridWidths(width, gap, 2, 40)
	lw, rw := cw, cw
	if stack {
		lw, rw = width, width
	}
	weekPanel := panel("◈ DAY OF WEEK", colCyan, lw, m.weekdayBars(ins, lw))
	projPanel := panel("◈ TOP PROJECTS", colCyan, rw, m.projectBars(rw, cur))
	row := arrangePanels(stack, gap, weekPanel, projPanel)

	return lipgloss.JoinVertical(lipgloss.Left, hero, hourPanel, row)
}

// hourStrip renders the 24-hour heat strip with axis labels and a legend.
func (m Model) hourStrip(width int) string {
	inner := width - 6
	cellW := inner / 24
	if cellW < 1 {
		cellW = 1
	}
	cells := heatCells(m.ins.HourHist[:], cellW)
	labels := map[int]string{}
	for _, h := range []int{0, 3, 6, 9, 12, 15, 18, 21} {
		labels[h*cellW] = fmt.Sprintf("%02d", h)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		cells,
		labelStyle.Render(axisLine(24*cellW, labels)),
		"",
		heatLegend(),
	)
}

func heatLegend() string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("idle "))
	for i := 0; i < len(heatRamp); i++ {
		b.WriteString(heatStyle(float64(i) / float64(len(heatRamp)-1)).Render("█"))
	}
	b.WriteString(labelStyle.Render(" busy"))
	return b.String()
}

func (m Model) weekdayBars(ins usage.Insights, width int) string {
	var max int64 = 1
	for _, v := range ins.WeekdayHist {
		if v > max {
			max = v
		}
	}
	barW := width - 6 - 4 - 9 // box + name(4) + tokens(9)
	if barW < 6 {
		barW = 6
	}
	var b strings.Builder
	for d := 0; d < 7; d++ {
		v := ins.WeekdayHist[d]
		fmt.Fprintf(&b, "%-4s%s %8s\n", weekdayNames[d], gauge(float64(v)/float64(max), barW), compact(v))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) projectBars(width int, cur string) string {
	buckets := usage.GroupByWith(m.events, m.reportOptions.Pricing, func(e usage.Event) string { return e.Project })
	if len(buckets) == 0 {
		return labelStyle.Render("no data")
	}
	if len(buckets) > 7 {
		buckets = buckets[:7]
	}
	maxTok := buckets[0].Totals.Total
	if maxTok < 1 {
		maxTok = 1
	}
	inner := width - 6
	barW := inner - 16 - 9 - 10 // name(16) + tokens(9) + cost(10)
	if barW < 6 {
		barW = 6
	}
	var b strings.Builder
	for _, bk := range buckets {
		name := truncate(bk.Key, 16)
		fmt.Fprintf(&b, "%-16s%s %8s %9s\n",
			name, gauge(float64(bk.Totals.Total)/float64(maxTok), barW),
			compact(bk.Totals.Total), fmt.Sprintf("%.2f %s", bk.Totals.CostUSD, cur))
	}
	return strings.TrimRight(b.String(), "\n")
}
