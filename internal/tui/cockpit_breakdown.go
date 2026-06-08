package tui

// breakdownView decomposes usage by agent, model, and speed. It shows the
// ENGINES share bar plus the models and speed tables side by side; zoom an
// item (enter) for the full per-agent clusters, all models, or the airspeed
// tape with every lane.
func (m Model) breakdownView(width int) string {
	prices := m.reportOptions.Pricing
	engines := panel("◈ ENGINES · share", colCyan, width, m.enginesBar(m.events, prices, width))

	if m.compact {
		return vstack(engines, panel("◈ MODELS · load", colCyan, width, m.modelsBody(width, 5)))
	}

	gap := 1
	cw, stack := gridWidths(width, gap, 2, 40)
	lw, rw := cw, cw
	if stack {
		lw, rw = width, width
	}
	models := panel("◈ MODELS · load", colCyan, lw, m.modelsBody(lw, 8))

	rows := speedRows(m.events)
	if len(rows) > 8 {
		rows = rows[:8]
	}
	maxTPS := 1.0
	if len(rows) > 0 && rows[0].tps > 0 {
		maxTPS = rows[0].tps
	}
	speed := panel("◈ OUTPUT SPEED", colCyan, rw, speedLanes(rows, maxTPS, rw))

	return vstack(engines, arrangePanels(stack, gap, models, speed))
}
