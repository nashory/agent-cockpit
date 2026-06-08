package tui

// breakdownView decomposes usage by model, agent, and speed. The hero is a
// 100%-stacked area chart of model share over time; below it are the model load
// table and the output-speed lanes. Zoom (enter) any of them, or the ENGINES
// per-agent clusters, for full detail.
func (m Model) breakdownView(width int) string {
	mix := panel("◈ MODEL MIX · 30d share", colCyan, width, m.modelStack(width, 8))

	if m.compact {
		return vstack(mix, panel("◈ MODELS · load", colCyan, width, m.modelsBody(width, 5)))
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

	return vstack(mix, arrangePanels(stack, gap, models, speed))
}
