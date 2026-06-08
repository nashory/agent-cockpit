package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

// contribRamp is a GitHub-style intensity ramp (idle → heavy) tuned for a dark
// terminal: a dim empty cell, then four brightening greens.
var contribRamp = []lipgloss.Color{
	lipgloss.Color("236"), // empty
	lipgloss.Color("22"),
	lipgloss.Color("28"),
	lipgloss.Color("34"),
	lipgloss.Color("46"),
}

// colEmptyCell is the faint dot for days with no activity.
var colEmptyCell = lipgloss.Color("240")

func contribStyle(level int) lipgloss.Style {
	if level < 0 {
		level = 0
	}
	if level >= len(contribRamp) {
		level = len(contribRamp) - 1
	}
	return lipgloss.NewStyle().Foreground(contribRamp[level])
}

// contribLevel maps a day's tokens to 0 (empty) or 1..4 by share of the busiest
// day in view.
func contribLevel(v, max int64) int {
	if v <= 0 {
		return 0
	}
	if max <= 0 {
		return 1
	}
	l := 1 + int(float64(v)/float64(max)*3.999)
	if l > 4 {
		l = 4
	}
	return l
}

func dayKey(t time.Time) string { return t.Format("2006-01-02") }

// dailyTokenMap sums total tokens per calendar day.
func dailyTokenMap(events []usage.Event) map[string]int64 {
	m := make(map[string]int64, len(events))
	for _, e := range events {
		if e.Timestamp.IsZero() {
			continue
		}
		m[dayKey(e.Timestamp)] += e.TotalTokens()
	}
	return m
}

// contributionGrid renders the year heat-grid body (month labels, weekday rows,
// legend, and the selected-day tooltip) and returns it with the week count.
func (m Model) contributionGrid(width int) (string, int) {
	tokMap := dailyTokenMap(m.events)

	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Fit the grid to the available width: prefer 2-wide square cells and a
	// full year, shrinking cells then week count on narrow terminals.
	const dayLabelW = 4
	inner := width - 6
	avail := inner - dayLabelW
	if avail < 8 {
		avail = 8
	}
	cellW := 2
	weeks := 52
	if avail < weeks*cellW {
		cellW = 1
	}
	if avail < weeks*cellW {
		weeks = avail / cellW
	}
	if weeks < 1 {
		weeks = 1
	}

	// start = the Sunday on/before the first visible week.
	start := end.AddDate(0, 0, -(weeks-1)*7)
	start = start.AddDate(0, 0, -int(start.Weekday()))

	// Selected day (cursor), navigable with arrows/hjkl.
	selDate := end.AddDate(0, 0, -m.calCursor)
	selKey := dayKey(selDate)

	// Busiest day in view sets the color scale.
	var max int64
	for c := 0; c < weeks; c++ {
		for r := 0; r < 7; r++ {
			d := start.AddDate(0, 0, c*7+r)
			if d.After(end) {
				continue
			}
			if v := tokMap[dayKey(d)]; v > max {
				max = v
			}
		}
	}

	// Month labels along the top, skipping crowded columns.
	monthLabels := map[int]string{}
	prevMonth := -1
	lastPos := -100
	for c := 0; c < weeks; c++ {
		d := start.AddDate(0, 0, c*7)
		mo := int(d.Month())
		if mo != prevMonth {
			prevMonth = mo
			if pos := c * cellW; pos-lastPos >= 4 {
				monthLabels[pos] = d.Format("Jan")
				lastPos = pos
			}
		}
	}
	monthLine := labelStyle.Render(strings.Repeat(" ", dayLabelW) + axisLine(weeks*cellW, monthLabels))

	// Seven weekday rows; label Mon/Wed/Fri like GitHub.
	dayLabels := map[int]string{1: "Mon", 3: "Wed", 5: "Fri"}
	rows := make([]string, 7)
	for r := 0; r < 7; r++ {
		var sb strings.Builder
		sb.WriteString(labelStyle.Render(fmt.Sprintf("%-*s", dayLabelW, dayLabels[r])))
		for c := 0; c < weeks; c++ {
			d := start.AddDate(0, 0, c*7+r)
			if d.After(end) {
				sb.WriteString(strings.Repeat(" ", cellW))
				continue
			}
			k := dayKey(d)
			v := tokMap[k]
			// Empty days are a faint dot, not a solid block, so "no activity"
			// reads clearly instead of looking like an unlabeled black box.
			glyph, style := "·", lipgloss.NewStyle().Foreground(colEmptyCell)
			if v > 0 {
				glyph, style = "█", contribStyle(contribLevel(v, max))
			}
			if k == selKey {
				style = style.Reverse(true) // cursor
			}
			sb.WriteString(style.Render(strings.Repeat(glyph, cellW)))
		}
		rows[r] = sb.String()
	}

	legend := contribLegend(cellW, max)
	tip := lipgloss.NewStyle().Foreground(colCyan).Render("◂ ") +
		valueStyle.Render(selDate.Format("Mon Jan 02 2006")) +
		labelStyle.Render(" · ") +
		lipgloss.NewStyle().Foreground(colGreen).Render(compact(tokMap[selKey])+" tokens") +
		lipgloss.NewStyle().Foreground(colCyan).Render(" ▸")
	grid := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{monthLine}, append(rows, "", legend, tip)...)...)
	return grid, weeks
}

// contribLegend explains the cells: a dot is no activity, the green ramp goes
// less -> more, annotated with the busiest day's token count.
func contribLegend(cellW int, max int64) string {
	block := strings.Repeat("█", cellW)
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(colEmptyCell).Render(strings.Repeat("·", cellW)))
	sb.WriteString(labelStyle.Render(" none    less "))
	for l := 1; l < len(contribRamp); l++ { // skip the empty level (handled above)
		sb.WriteString(contribStyle(l).Render(block))
		sb.WriteString(" ")
	}
	sb.WriteString(labelStyle.Render("more    each cell = 1 day · peak " + compact(max) + " tokens/day"))
	return sb.String()
}

// calendarHero summarizes the visible year (52 weeks): totals, active days, and
// streaks.
func (m Model) calendarHero(tokMap map[string]int64) string {
	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := end.AddDate(0, 0, -(52-1)*7)
	start = start.AddDate(0, 0, -int(start.Weekday()))

	var total, busiest int64
	var busiestDay time.Time
	active, longest, cur := 0, 0, 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		v := tokMap[dayKey(d)]
		if v > 0 {
			active++
			total += v
			cur++
			if cur > longest {
				longest = cur
			}
			if v > busiest {
				busiest = v
				busiestDay = d
			}
		} else {
			cur = 0
		}
	}
	current := cur // run ending at today (0 if today idle)

	peak := "n/a"
	if !busiestDay.IsZero() {
		peak = busiestDay.Format("Jan 02")
	}
	cells := []string{
		readout("TOKENS", compact(total), colCyan),
		readout("ACTIVE", fmt.Sprintf("%d d", active), colGreen),
		readout("CURRENT", fmt.Sprintf("%d d", current), colAmber),
	}
	if !m.compact {
		cells = append(cells,
			readout("LONGEST", fmt.Sprintf("%d d", longest), colGreen),
			readout("PEAK DAY", peak, colText),
			readout("PEAK TOK", compact(busiest), colText),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, spread(cells, "   ")...)
}
