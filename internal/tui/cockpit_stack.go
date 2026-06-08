package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
)

// modelPalette gives the top models distinct bands; the rest fold into "other".
var modelPalette = []lipgloss.Color{
	lipgloss.Color("51"),  // cyan
	lipgloss.Color("213"), // magenta
	lipgloss.Color("48"),  // green
	lipgloss.Color("220"), // amber
	lipgloss.Color("75"),  // blue
	lipgloss.Color("208"), // orange
}

var colOther = lipgloss.Color("240")

// colIdle marks days with no activity at all; rendered as a solid white band
// (same block as the model bands, just white) so an empty day reads cleanly.
var colIdle = lipgloss.Color("231")

const stackTopModels = 5

// modelMix groups events into the top-N models (plus "other") and returns their
// per-day token counts over the last `days` days, ranked by total descending.
func modelMix(events []usage.Event, days int) (labels []string, colors []lipgloss.Color, daily [][]int64, totals []int64, grand int64) {
	if days < 1 {
		days = 1
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1))

	perDay := map[string][]int64{}
	perTot := map[string]int64{}
	for _, e := range events {
		if e.Timestamp.IsZero() {
			continue
		}
		idx := int(e.Timestamp.Truncate(24*time.Hour).Sub(start) / (24 * time.Hour))
		if idx < 0 || idx >= days {
			continue
		}
		mdl := e.Model
		if mdl == "" {
			mdl = "unknown"
		}
		if perDay[mdl] == nil {
			perDay[mdl] = make([]int64, days)
		}
		v := e.TotalTokens()
		perDay[mdl][idx] += v
		perTot[mdl] += v
		grand += v
	}

	names := make([]string, 0, len(perTot))
	for n := range perTot {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if perTot[names[i]] != perTot[names[j]] {
			return perTot[names[i]] > perTot[names[j]]
		}
		return names[i] < names[j]
	})

	other := make([]int64, days)
	var otherTot int64
	for rank, n := range names {
		if rank < stackTopModels {
			labels = append(labels, n)
			colors = append(colors, modelPalette[rank%len(modelPalette)])
			daily = append(daily, perDay[n])
			totals = append(totals, perTot[n])
			continue
		}
		for d := 0; d < days; d++ {
			other[d] += perDay[n][d]
		}
		otherTot += perTot[n]
	}
	if otherTot > 0 {
		labels = append(labels, "other")
		colors = append(colors, colOther)
		daily = append(daily, other)
		totals = append(totals, otherTot)
	}
	return labels, colors, daily, totals, grand
}

// stackBands distributes a column's per-group tokens into exactly H rows using
// largest-remainder rounding, so the bands always fill the full height.
func stackBands(daily [][]int64, col int, total int64, h int) []int {
	n := len(daily)
	rows := make([]int, n)
	used := 0
	for g := 0; g < n; g++ {
		rows[g] = int(float64(daily[g][col]) / float64(total) * float64(h))
		used += rows[g]
	}
	for left := h - used; left > 0; left-- {
		best, bestDeficit := -1, 0.0
		for g := 0; g < n; g++ {
			if daily[g][col] == 0 {
				continue
			}
			deficit := float64(daily[g][col])/float64(total)*float64(h) - float64(rows[g])
			if best == -1 || deficit > bestDeficit {
				best, bestDeficit = g, deficit
			}
		}
		if best == -1 {
			best = 0
		}
		rows[best]++
	}
	return rows
}

// modelStack renders a 100%-stacked area chart of model token share over time:
// one column per day, each column split top-to-bottom by model proportion.
func (m Model) modelStack(width, height int) string {
	const gutter = 4 // left y-axis label column ("100 ")
	inner := width - 6
	if inner < gutter+12 || height < 4 {
		return labelStyle.Render("· widen to view ·")
	}
	if height > 24 {
		height = 24
	}
	plotW := inner - gutter
	colW := 1
	if plotW >= 60 {
		colW = 2
	}
	days := plotW / colW
	if days > 30 {
		days = 30
	}
	if days < 1 {
		days = 1
	}
	usedW := days * colW

	labels, colors, daily, totals, grand := modelMix(m.events, days)
	if grand == 0 {
		return labelStyle.Render("no data")
	}

	// Band index per row (bottom-up) for each column.
	bandOf := make([][]int, days)
	for c := 0; c < days; c++ {
		col := make([]int, height)
		for r := range col {
			col[r] = -1
		}
		var total int64
		for g := range daily {
			total += daily[g][c]
		}
		if total > 0 {
			rows := stackBands(daily, c, total, height)
			r := 0
			for g := 0; g < len(rows) && r < height; g++ {
				for k := 0; k < rows[g] && r < height; k++ {
					col[r] = g
					r++
				}
			}
		}
		bandOf[c] = col
	}

	var b strings.Builder
	for sr := 0; sr < height; sr++ {
		rfb := height - 1 - sr // row from bottom
		lab := ""
		switch sr {
		case 0:
			lab = "100"
		case height / 2:
			lab = "50"
		case height - 1:
			lab = "0"
		}
		b.WriteString(labelStyle.Render(fmt.Sprintf("%3s ", lab)))
		for c := 0; c < days; c++ {
			if g := bandOf[c][rfb]; g >= 0 {
				b.WriteString(lipgloss.NewStyle().Foreground(colors[g]).Render(strings.Repeat("█", colW)))
			} else {
				b.WriteString(lipgloss.NewStyle().Foreground(colIdle).Render(strings.Repeat("█", colW)))
			}
		}
		b.WriteByte('\n')
	}

	// X-axis date ticks.
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1))
	xl := map[int]string{}
	for _, dd := range []int{0, days / 2, days - 1} {
		if dd >= 0 && dd < days {
			xl[dd*colW] = start.AddDate(0, 0, dd).Format("01/02")
		}
	}
	b.WriteString(strings.Repeat(" ", gutter) + labelStyle.Render(axisLine(usedW, xl)) + "\n")

	// Legend: swatch + model + overall share.
	var leg strings.Builder
	for i := range labels {
		share := float64(totals[i]) / float64(grand) * 100
		leg.WriteString(lipgloss.NewStyle().Foreground(colors[i]).Render("█"))
		leg.WriteString(labelStyle.Render(fmt.Sprintf(" %s %.0f%%   ", truncate(labels[i], 16), share)))
	}
	leg.WriteString(lipgloss.NewStyle().Foreground(colIdle).Render("█"))
	leg.WriteString(labelStyle.Render(" no activity"))
	b.WriteString(strings.Repeat(" ", gutter) + strings.TrimRight(leg.String(), " "))
	return b.String()
}
