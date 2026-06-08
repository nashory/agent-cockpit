package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// zoomTarget is one focusable widget on a tab: navigate with arrows/hjkl, press
// enter to render it fullscreen.
type zoomTarget struct {
	title  string
	accent lipgloss.Color
	render func(w, h int) string
}

// chartH clamps a fullscreen chart height to a sane braille range.
func chartH(h int) int {
	h -= 2
	if h < 6 {
		return 6
	}
	if h > 30 {
		return 30
	}
	return h
}

// zoomTargets lists the focusable widgets for the current tab, each able to
// render itself fullscreen. The renderers reuse the same body helpers as the
// dashboard, with charts honoring the available height.
func (m Model) zoomTargets() []zoomTarget {
	ev := m.events
	prices := m.reportOptions.Pricing
	cur := m.currency()
	tokChart := func(w, h int) string { return seriesChart(dailySeries(ev, 30), w, chartH(h), colGreen) }
	costChart := func(w, h int) string { return seriesChart(dailyCostSeries(ev, 30, prices), w, chartH(h), colAmber) }
	enginesZoom := func(w, h int) string { return m.enginesBar(ev, prices, w) }
	modelsZoom := func(w, h int) string { return m.modelsBody(w, 0) }

	switch m.view {
	case overview:
		return []zoomTarget{
			{"TREND", colCyan, tokChart},
			{"ENGINES", colCyan, enginesZoom},
			{"MODELS", colCyan, modelsZoom},
			{"ACTIVITY", colCyan, func(w, h int) string { return m.agentSparks(ev, w) }},
		}
	case breakdown:
		return []zoomTarget{
			{"ENGINES", colCyan, func(w, h int) string { return m.agentClusters(w) }},
			{"MODELS", colCyan, modelsZoom},
			{"SPEED", colCyan, func(w, h int) string {
				rows := speedRows(ev)
				maxTPS := 1.0
				if len(rows) > 0 && rows[0].tps > 0 {
					maxTPS = rows[0].tps
				}
				return lipgloss.JoinVertical(lipgloss.Left, m.airspeedHero(w), "", speedLanes(rows, maxTPS, w))
			}},
			{"MODEL MIX", colCyan, func(w, h int) string { return m.modelStack(w, h) }},
		}
	case trends:
		return []zoomTarget{
			{"TOKENS", colCyan, tokChart},
			{"COST", colAmber, costChart},
			{"EFFICIENCY", colGreen, func(w, h int) string { return m.efficiencyBody(w - 6) }},
			{"ECONOMICS", colAmber, func(w, h int) string { return m.economicsBody(w - 6) }},
			{"CADENCE", colCyan, func(w, h int) string { return m.cadenceBody() }},
		}
	case activity:
		return []zoomTarget{
			{"CALENDAR", colGreen, func(w, h int) string {
				g, _ := m.contributionGrid(w)
				return g
			}},
			{"HOUR OF DAY", colCyan, func(w, h int) string { return m.hourStrip(w) }},
			{"DAY OF WEEK", colCyan, func(w, h int) string { return m.weekdayBars(m.ins, w) }},
			{"PROJECTS", colCyan, func(w, h int) string { return m.projectBars(w, cur) }},
		}
	}
	return nil
}

// focusBar is the widget selector shown above the dashboard: chips for each
// zoom target with the focused one highlighted.
func (m Model) focusBar(targets []zoomTarget) string {
	if len(targets) == 0 {
		return ""
	}
	chips := make([]string, 0, len(targets))
	for i, t := range targets {
		if i == m.focus {
			chips = append(chips, focusChip.Render(" "+t.title+" "))
		} else {
			chips = append(chips, labelStyle.Render(" "+t.title+" "))
		}
	}
	sep := lipgloss.NewStyle().Foreground(colDim).Render("·")
	row := chips[0]
	for _, c := range chips[1:] {
		row += sep + c
	}
	return labelStyle.Render("FOCUS ") + row + labelStyle.Render("   ↑↓ select · enter zoom")
}

// zoomedContent renders the focused widget fullscreen with a header.
func (m Model) zoomedContent(width, height int, targets []zoomTarget) string {
	t := targets[m.focus]
	body := t.render(width, height)
	return heroPanel("⤢ "+t.title+"   ·   esc back", t.accent, width, body)
}
