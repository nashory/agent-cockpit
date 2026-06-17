package tui

import (
	"fmt"
	"strings"
	"time"

	tslc "github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

type costDayRow struct {
	date   time.Time
	cost   float64
	ma7    float64
	ma14   float64
	ma30   float64
	tokens int64
	source string
	model  string
}

func (m Model) costView(width int) string {
	days := m.costWindowDays()
	rows := m.costRows(days)
	hero := heroPanel("✈ COST RATE · "+fmt.Sprintf("%dd", days), colAmber, width, m.costHero(rows))
	gap := 1
	cellW, stack := gridWidths(width, gap, 2, 38)
	if stack {
		cellW = width
	}
	cellH := m.costGridCellHeight()
	row1 := panelsRow(stack, gap,
		panelSpec{"◈ SPEND RATE · daily bars + 7d avg", colAmber, cellW, m.costSpendRateBars(rows, cellW, cellH)},
		panelSpec{"◈ TREND · 7d vs 30d", colGreen, cellW, m.costTrendChart(rows, cellW, cellH)},
	)
	row2 := panelsRow(stack, gap,
		panelSpec{"◈ CROSSOVER · 7d minus 30d", colCyan, cellW, m.costCrossoverChart(rows, cellW, cellH)},
		panelSpec{"◈ MONTH PACE · actual vs 30d run-rate", colAmber, cellW, m.costPaceChart(rows, cellW, cellH)},
	)
	return vstack(hero, row1, row2)
}

func (m Model) costGridCellHeight() int {
	if m.height <= 0 {
		return 10
	}
	h := (m.height - 21) / 2
	if h < 8 {
		return 8
	}
	if h > 16 {
		return 16
	}
	return h
}

func (m Model) costWindowDays() int {
	if m.windowDays < 30 {
		return 30
	}
	return m.windowDays
}

func (m Model) costRows(days int) []costDayRow {
	if days < 1 {
		days = 1
	}
	prices := m.reportOptions.Pricing
	start := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)
	rows := make([]costDayRow, days)
	topCostByDay := make([]map[string]float64, days)
	for i := range rows {
		rows[i].date = start.AddDate(0, 0, i)
		topCostByDay[i] = map[string]float64{}
	}
	for _, e := range m.events {
		if e.Timestamp.IsZero() {
			continue
		}
		idx := int(e.Timestamp.Truncate(24*time.Hour).Sub(start) / (24 * time.Hour))
		if idx < 0 || idx >= days {
			continue
		}
		cost := usage.EstimateCostWith(e, prices)
		rows[idx].cost += cost
		rows[idx].tokens += e.TotalTokens()
		key := strings.TrimSpace(e.Source + " / " + shortModel(e.Model))
		if key == "/" || key == "" {
			key = "unknown"
		}
		topCostByDay[idx][key] += cost
	}
	costs := make([]float64, days)
	for i, row := range rows {
		costs[i] = row.cost
		var topKey string
		var topCost float64
		for key, cost := range topCostByDay[i] {
			if cost > topCost {
				topCost = cost
				topKey = key
			}
		}
		if topKey != "" {
			parts := strings.SplitN(topKey, " / ", 2)
			rows[i].source = parts[0]
			if len(parts) > 1 {
				rows[i].model = parts[1]
			}
		}
	}
	ma7 := movingAverage(costs, 7)
	ma14 := movingAverage(costs, 14)
	ma30 := movingAverage(costs, 30)
	for i := range rows {
		rows[i].ma7 = ma7[i]
		rows[i].ma14 = ma14[i]
		rows[i].ma30 = ma30[i]
	}
	return rows
}

func movingAverage(values []float64, window int) []float64 {
	if window < 1 {
		window = 1
	}
	out := make([]float64, len(values))
	var sum float64
	for i, v := range values {
		sum += v
		if i >= window {
			sum -= values[i-window]
		}
		n := i + 1
		if n > window {
			n = window
		}
		out[i] = sum / float64(n)
	}
	return out
}

func (m Model) costHero(rows []costDayRow) string {
	if len(rows) == 0 {
		return labelStyle.Render("no data")
	}
	cur := m.currency()
	var total, peak float64
	for _, row := range rows {
		total += row.cost
		if row.cost > peak {
			peak = row.cost
		}
	}
	last := rows[len(rows)-1]
	return lipgloss.JoinHorizontal(lipgloss.Top, spread([]string{
		readout("TODAY", fmt.Sprintf("~%.2f %s", last.cost, cur), colAmber),
		readout("7D AVG", fmt.Sprintf("~%.2f", last.ma7), colGreen),
		readout("14D AVG", fmt.Sprintf("~%.2f", last.ma14), colCyan),
		readout("30D AVG", fmt.Sprintf("~%.2f", last.ma30), colText),
		readout("MONTH PACE", fmt.Sprintf("~%.2f %s", last.ma30*30, cur), colAmber),
		readout("PEAK", fmt.Sprintf("~%.2f", peak), colGreen),
		readout("TOTAL", fmt.Sprintf("~%.2f", total), colText),
	}, "   ")...)
}

func (m Model) costZoom(kind string, width, height int) string {
	days := m.costWindowDays()
	rows := m.costRows(days)
	if len(rows) == 0 {
		return labelStyle.Render("no data")
	}
	sel := m.costSelectedIndex(len(rows))
	if kind == "spend" && m.costPopup {
		return m.costSelectedDayBreakdown(rows, sel, width)
	}
	chartHeight := height - 12
	if chartHeight < 8 {
		chartHeight = 8
	}
	if chartHeight > 30 {
		chartHeight = 30
	}

	var chart string
	switch kind {
	case "spend":
		chart = m.costSpendRateBarsSelected(rows, width, chartHeight, sel)
	case "trend":
		chart = m.costTrendChartSelected(rows, width, chartHeight, sel)
	case "crossover":
		chart = m.costCrossoverChartSelected(rows, width, chartHeight, sel)
	case "pace":
		chart = m.costPaceChartSelected(rows, width, chartHeight, sel)
	default:
		chart = m.costTrendChartSelected(rows, width, chartHeight, sel)
	}

	controls := "←/→ inspect day · home/end jump · w window · esc back"
	if kind == "spend" {
		controls = "←/→ inspect day · enter breakdown · home/end jump · w window · esc back"
	}
	parts := []string{
		labelStyle.Render(controls),
		chart,
		"",
		m.costZoomInspector(rows, sel, width, kind),
	}
	tableRows := height - chartHeight - 12
	if tableRows > 3 {
		if tableRows > 8 {
			tableRows = 8
		}
		parts = append(parts, "", m.costTable(rows, width, tableRows))
	}
	return strings.TrimRight(lipgloss.JoinVertical(lipgloss.Left, parts...), "\n")
}

func (m Model) costSelectedDayBreakdown(rows []costDayRow, idx int, width int) string {
	if idx < 0 || idx >= len(rows) {
		return labelStyle.Render("no data")
	}
	row := rows[idx]
	start := time.Date(row.date.Year(), row.date.Month(), row.date.Day(), 0, 0, 0, 0, row.date.Location())
	end := start.AddDate(0, 0, 1)
	var dayEvents []usage.Event
	for _, e := range m.events {
		if !e.Timestamp.IsZero() && !e.Timestamp.Before(start) && e.Timestamp.Before(end) {
			dayEvents = append(dayEvents, e)
		}
	}
	title := fmt.Sprintf("BREAKDOWN · %s · %d events · esc chart", row.date.Format("2006-01-02 Mon"), len(dayEvents))
	body := m.usageDetailBody(dayEvents)
	panelW := width - 6
	if panelW < 40 {
		panelW = width
	}
	return panel(title, colAmber, panelW, body)
}

func (m Model) costSelectedIndex(total int) int {
	if total <= 0 {
		return 0
	}
	idx := m.trendSel
	if idx < 0 {
		return 0
	}
	if idx >= total {
		return total - 1
	}
	return idx
}

func (m Model) costZoomInspector(rows []costDayRow, idx int, width int, kind string) string {
	row := rows[idx]
	cur := m.currency()
	driver := strings.TrimSpace(row.source + " / " + row.model)
	if driver == "/" || driver == "" {
		driver = "no spend"
	}
	prev := row.cost
	if idx > 0 {
		prev = rows[idx-1].cost
	}
	dayDelta := row.cost - prev
	cross := row.ma7 - row.ma30
	monthActual := costMonthActual(rows, idx)
	monthPace := row.ma30 * float64(row.date.Day())
	var note string
	switch kind {
	case "spend":
		note = fmt.Sprintf("1d delta %+0.2f %s vs previous day", dayDelta, cur)
	case "trend":
		note = fmt.Sprintf("7d trend is %+0.2f %s vs 30d baseline", cross, cur)
	case "crossover":
		note = fmt.Sprintf("crossover %+0.2f %s; positive means recent spend is above baseline", cross, cur)
	case "pace":
		note = fmt.Sprintf("month actual %.2f %s vs 30d run-rate %.2f", monthActual, cur, monthPace)
	default:
		note = fmt.Sprintf("top driver %s", driver)
	}
	cells := spread([]string{
		readout("DATE", row.date.Format("2006-01-02"), colText),
		readout("1D", fmt.Sprintf("%.2f %s", row.cost, cur), colAmber),
		readout("7D AVG", fmt.Sprintf("%.2f", row.ma7), colGreen),
		readout("14D AVG", fmt.Sprintf("%.2f", row.ma14), colCyan),
		readout("30D AVG", fmt.Sprintf("%.2f", row.ma30), colText),
		readout("TOKENS", compact(row.tokens), colCyan),
	}, "   ")
	inner := width - 6
	if inner < 20 {
		inner = 20
	}
	detail := labelStyle.Render(truncate(note+" · driver "+driver, inner))
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, cells...),
		detail,
	)
}

func costMonthActual(rows []costDayRow, idx int) float64 {
	if idx < 0 || idx >= len(rows) {
		return 0
	}
	month := rows[idx].date.Month()
	year := rows[idx].date.Year()
	var total float64
	for i := 0; i <= idx; i++ {
		if rows[i].date.Month() == month && rows[i].date.Year() == year {
			total += rows[i].cost
		}
	}
	return total
}

func (m Model) costMovingAverageChart(rows []costDayRow, width, height int) string {
	return m.costSpendRateChart(rows, width, height)
}

type costLine struct {
	name  string
	color lipgloss.Color
	value func(costDayRow, int, []costDayRow) float64
}

func costMultiLineChart(rows []costDayRow, width, height int, lines []costLine) string {
	return costMultiLineChartSelected(rows, width, height, lines, -1, colCyan)
}

func costMultiLineChartSelected(rows []costDayRow, width, height int, lines []costLine, selected int, cursorColor lipgloss.Color) string {
	if len(rows) == 0 {
		return labelStyle.Render("no data")
	}
	inner := width - 6
	if inner < 20 {
		inner = 20
	}
	var maxY, minY float64
	for i, row := range rows {
		for _, line := range lines {
			v := line.value(row, i, rows)
			if v > maxY {
				maxY = v
			}
			if v < minY {
				minY = v
			}
		}
	}
	if maxY <= minY {
		maxY = minY + 1
	}
	start := rows[0].date
	end := rows[len(rows)-1].date
	tc := tslc.New(inner, height,
		tslc.WithTimeRange(start, end),
		tslc.WithYRange(minY, maxY),
		tslc.WithAxesStyles(labelStyle, labelStyle),
	)
	tc.SetViewTimeAndYRange(start, end, minY, maxY)
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		names = append(names, line.name)
		tc.SetDataSetStyle(line.name, lipgloss.NewStyle().Foreground(line.color))
		for i, row := range rows {
			tc.PushDataSet(line.name, tslc.TimePoint{Time: row.date, Value: line.value(row, i, rows)})
		}
	}
	tc.DrawBrailleDataSets(names)
	chart := paddedChart(tc.View())
	if selected >= 0 {
		chart = lipgloss.JoinVertical(lipgloss.Left, chart, costChartCursor(chart, len(rows), selected, cursorColor))
	}
	parts := make([]string, 0, len(lines)*2)
	for i, line := range lines {
		if i > 0 {
			parts = append(parts, labelStyle.Render("  "))
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(line.color).Render(line.name))
	}
	legend := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return lipgloss.JoinVertical(lipgloss.Left, chart, legend)
}

func costChartCursor(chart string, total, selected int, color lipgloss.Color) string {
	if total < 1 {
		total = 1
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}
	gutter, plotW := 0, 1
	for _, ln := range strings.Split(chart, "\n") {
		if i := strings.IndexRune(ln, '└'); i >= 0 {
			runes := []rune(ln)
			for j, rn := range runes {
				if rn == '└' {
					gutter = j + 1
					plotW = len(runes) - gutter
					break
				}
			}
			break
		}
	}
	if plotW < 1 {
		plotW = 1
	}
	pos := gutter
	if total > 1 {
		pos += selected * (plotW - 1) / (total - 1)
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(strings.Repeat(" ", pos) + "▲")
}

func (m Model) costSpendRateChart(rows []costDayRow, width, height int) string {
	return m.costSpendRateBars(rows, width, height)
}

func (m Model) costSpendRateBars(rows []costDayRow, width, limit int) string {
	return m.costSpendRateBarsSelected(rows, width, limit, -1)
}

func (m Model) costSpendRateBarsSelected(rows []costDayRow, width, limit, selected int) string {
	if len(rows) == 0 {
		return labelStyle.Render("no data")
	}
	inner := width - 6
	if inner < 30 {
		inner = 30
	}
	start := costVisibleStart(len(rows), limit, selected)
	end := start + limit
	if end > len(rows) {
		end = len(rows)
	}
	var maxCost float64
	for i := start; i < end; i++ {
		if rows[i].cost > maxCost {
			maxCost = rows[i].cost
		}
	}
	if maxCost <= 0 {
		maxCost = 1
	}
	barW := inner - 28
	if barW < 8 {
		barW = 8
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", labelStyle.Render(fmt.Sprintf("%-9s %-*s %8s %7s", "DAY", barW, "1D COST", "COST", "7D AVG")))
	for i := start; i < end; i++ {
		row := rows[i]
		line := fmt.Sprintf("%-9s %s %8.2f %7.2f",
			row.date.Format("01-02 Mon"),
			gaugeColored(row.cost/maxCost, barW, colAmber),
			row.cost,
			row.ma7)
		if i == selected {
			line = lipgloss.NewStyle().Reverse(true).Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func costVisibleStart(total, limit, selected int) int {
	if total <= 0 {
		return 0
	}
	if limit < 1 {
		limit = 1
	}
	if limit >= total {
		return 0
	}
	if selected < 0 {
		return total - limit
	}
	if selected >= total {
		selected = total - 1
	}
	start := selected - limit/2
	if start < 0 {
		return 0
	}
	maxStart := total - limit
	if start > maxStart {
		return maxStart
	}
	return start
}

func (m Model) costTrendChart(rows []costDayRow, width, height int) string {
	return m.costTrendChartSelected(rows, width, height, -1)
}

func (m Model) costTrendChartSelected(rows []costDayRow, width, height, selected int) string {
	return costMultiLineChartSelected(rows, width, height, []costLine{
		{"30d baseline", colDim, func(r costDayRow, _ int, _ []costDayRow) float64 { return r.ma30 }},
		{"7d trend", colGreen, func(r costDayRow, _ int, _ []costDayRow) float64 { return r.ma7 }},
	}, selected, colGreen)
}

func (m Model) costCrossoverChart(rows []costDayRow, width, height int) string {
	return m.costCrossoverChartSelected(rows, width, height, -1)
}

func (m Model) costCrossoverChartSelected(rows []costDayRow, width, height, selected int) string {
	return costMultiLineChartSelected(rows, width, height, []costLine{
		{"zero", colDim, func(costDayRow, int, []costDayRow) float64 { return 0 }},
		{"7d-30d", colAmber, func(r costDayRow, _ int, _ []costDayRow) float64 { return r.ma7 - r.ma30 }},
	}, selected, colCyan)
}

func (m Model) costPaceChart(rows []costDayRow, width, height int) string {
	return m.costPaceChartSelected(rows, width, height, -1)
}

func (m Model) costPaceChartSelected(rows []costDayRow, width, height, selected int) string {
	return costMultiLineChartSelected(rows, width, height, []costLine{
		{"actual", colAmber, func(_ costDayRow, i int, rows []costDayRow) float64 {
			var total float64
			month := rows[i].date.Month()
			year := rows[i].date.Year()
			for j := 0; j <= i; j++ {
				if rows[j].date.Month() == month && rows[j].date.Year() == year {
					total += rows[j].cost
				}
			}
			return total
		}},
		{"30d pace", colDim, func(r costDayRow, _ int, _ []costDayRow) float64 {
			return r.ma30 * float64(r.date.Day())
		}},
	}, selected, colAmber)
}

func (m Model) costEfficiencyChart(rows []costDayRow, width, height int) string {
	values := make([]float64, len(rows))
	for i, row := range rows {
		if row.tokens > 0 {
			values[i] = row.cost / float64(row.tokens) * 1000
		}
	}
	ma7 := movingAverage(values, 7)
	ma30 := movingAverage(values, 30)
	return costMultiLineChart(rows, width, height, []costLine{
		{"1d", colAmber, func(_ costDayRow, i int, _ []costDayRow) float64 { return values[i] }},
		{"7d", colGreen, func(_ costDayRow, i int, _ []costDayRow) float64 { return ma7[i] }},
		{"30d", colDim, func(_ costDayRow, i int, _ []costDayRow) float64 { return ma30[i] }},
	})
}

func (m Model) costTable(rows []costDayRow, width, limit int) string {
	if len(rows) == 0 {
		return labelStyle.Render("no data")
	}
	cur := m.currency()
	inner := width - 6
	if inner < 40 {
		inner = 40
	}
	const dateW, costW, avgW, tokW = 10, 10, 9, 9
	driverW := inner - dateW - costW - avgW*3 - tokW - 6
	if driverW < 8 {
		driverW = 8
	}
	line := func(date, cost, ma7, ma14, ma30, tok, driver string) string {
		return fmt.Sprintf("%-*s %*s %*s %*s %*s %*s %-*s",
			dateW, date, costW, cost, avgW, ma7, avgW, ma14, avgW, ma30, tokW, tok,
			driverW, truncate(driver, driverW))
	}
	var b strings.Builder
	b.WriteString(labelStyle.Render(line("DATE", "1D", "7D", "14D", "30D", "TOKENS", "TOP DRIVER")))
	b.WriteByte('\n')
	start := len(rows) - limit
	if start < 0 {
		start = 0
	}
	for i := len(rows) - 1; i >= start; i-- {
		row := rows[i]
		driver := strings.TrimSpace(row.source + " / " + row.model)
		if driver == "/" || driver == "" {
			driver = "no spend"
		}
		b.WriteString(line(row.date.Format("01-02 Mon"),
			fmt.Sprintf("%.2f", row.cost),
			fmt.Sprintf("%.2f", row.ma7),
			fmt.Sprintf("%.2f", row.ma14),
			fmt.Sprintf("%.2f", row.ma30),
			compact(row.tokens),
			driver,
		))
		if i == len(rows)-1 {
			b.WriteString(labelStyle.Render("  " + cur))
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
