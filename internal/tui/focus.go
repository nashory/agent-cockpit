package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/usage"
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
// render itself fullscreen. The list is kept in lockstep with what the tab body
// actually draws: the order follows the on-screen reading order, and compact
// mode returns only the subset of panels the compact layout renders, so the
// FOCUS bar never advertises a widget that isn't on screen.
func (m Model) zoomTargets() []zoomTarget {
	ev := m.events
	prices := m.reportOptions.Pricing
	cur := m.currency()
	tokChart := func(w, h int) string { return seriesChart(dailySeries(ev, 30), w, chartH(h), colGreen) }
	costChart := func(w, h int) string { return seriesChart(dailyCostSeries(ev, 30, prices), w, chartH(h), colAmber) }
	thrChart := func(w, h int) string { return seriesChart(dailyThroughput(ev, 30), w, chartH(h), colCyan) }
	velChart := func(w, h int) string { return seriesChart(dailyVelocity(ev, 30), w, chartH(h), colCyan) }
	enginesBar := func(w, h int) string { return m.enginesBar(ev, prices, w) }
	enginesClusters := func(w, h int) string { return m.agentClusters(w) }
	modelsZoom := func(w, h int) string { return m.modelsBody(w, 0) }
	trend := zoomTarget{"TREND", colGreen, tokChart}
	engines := zoomTarget{"ENGINES", colCyan, enginesBar}
	models := zoomTarget{"MODELS", colCyan, modelsZoom}
	activitySpark := zoomTarget{"ACTIVITY", colCyan, func(w, h int) string { return m.agentSparks(ev, w) }}
	mix := zoomTarget{"MODEL MIX", colCyan, func(w, h int) string { return m.modelStack(w, h) }}
	speed := zoomTarget{"OUTPUT SPEED", colCyan, func(w, h int) string {
		rows := speedRows(ev)
		maxTPS := 1.0
		if len(rows) > 0 && rows[0].tps > 0 {
			maxTPS = rows[0].tps
		}
		return lipgloss.JoinVertical(lipgloss.Left, m.airspeedHero(w), "", speedLanes(rows, maxTPS, w))
	}}
	tokens := zoomTarget{"TOKENS", colGreen, tokChart}
	cal := zoomTarget{"CALENDAR", colGreen, func(w, h int) string {
		g, _ := m.contributionGrid(w)
		return g
	}}

	switch m.view {
	case overview:
		// Body: ENGINES | TREND, then MODELS | ACTIVITY. Compact: ENGINES, TREND.
		if m.compact {
			return []zoomTarget{engines, trend}
		}
		return []zoomTarget{engines, trend, models, activitySpark}
	case breakdown:
		// Body: ENGINES, then MODELS | SPEED, then MODEL MIX.
		// Compact: ENGINES, MODELS, MODEL MIX (no SPEED lane).
		engines.render = enginesClusters // zoom shows the per-agent clusters
		if m.compact {
			return []zoomTarget{engines, models, mix}
		}
		return []zoomTarget{engines, models, speed, mix}
	case trends:
		// Body: TOKENS | COST, then EFFICIENCY | ECONOMICS | CADENCE.
		// Compact: TOKENS only.
		if m.compact {
			return []zoomTarget{tokens}
		}
		return []zoomTarget{
			tokens,
			{"COST", colAmber, costChart},
			{"VELOCITY", colCyan, velChart},
			{"THROUGHPUT", colCyan, thrChart},
			{"EFFICIENCY", colGreen, func(w, h int) string { return m.efficiencyBody(w - 6) }},
			{"ECONOMICS", colAmber, func(w, h int) string { return m.economicsBody(w - 6) }},
			{"CADENCE", colGreen, func(w, h int) string { return m.cadenceBody() }},
		}
	case activity:
		// Body: CALENDAR, then HOUR OF DAY, then DAY OF WEEK | TOP PROJECTS.
		// Compact: CALENDAR only.
		if m.compact {
			return []zoomTarget{cal}
		}
		return []zoomTarget{
			cal,
			{"HOUR OF DAY", colCyan, func(w, h int) string { return m.hourStrip(w) }},
			{"DAY OF WEEK", colCyan, func(w, h int) string { return m.weekdayBars(m.ins, w) }},
			{"TOP PROJECTS", colCyan, func(w, h int) string { return m.projectBars(w, cur) }},
		}
	case daily:
		// One table; zoom shows as many day rows as the height allows.
		return []zoomTarget{
			{"DAILY", colCyan, func(w, h int) string {
				limit := h - 4
				if limit < 6 {
					limit = 6
				}
				return m.ledgerTable(w, limit)
			}},
		}
	case blocks:
		return []zoomTarget{
			{"BLOCKS", colCyan, func(w, h int) string {
				limit := h - 4
				if limit < 6 {
					limit = 6
				}
				return m.blocksTable(usage.SessionBlocks(ev, prices, usage.DefaultBlockWindow), w, limit)
			}},
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
