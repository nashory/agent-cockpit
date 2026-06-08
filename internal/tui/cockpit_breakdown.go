package tui

// breakdownView decomposes usage by agent, model, and speed: the ENGINES share
// bar on top, the model load table and output-speed lanes in the middle, and a
// 100%-stacked area chart of model mix over time at the bottom. Zoom (enter) any
// of them for full detail.
func (m Model) breakdownView(width int) string {
	prices := m.reportOptions.Pricing
	span := m.dataSpanLabel()
	engines := panel("◈ ENGINES · share · "+span, colCyan, width, m.enginesBar(m.events, prices, width))
	mix := panel("◈ MODEL MIX · 30d share", colCyan, width, m.modelStack(width, 8))

	if m.compact {
		return vstack(engines, panel("◈ MODELS · load · "+span, colCyan, width, m.modelsBody(width, 5)), mix)
	}

	gap := 1
	cw, stack := gridWidths(width, gap, 2, 40)
	lw, rw := cw, cw
	if stack {
		lw, rw = width, width
	}
	rows := speedRows(m.events)
	if len(rows) > 8 {
		rows = rows[:8]
	}
	maxTPS := 1.0
	if len(rows) > 0 && rows[0].tps > 0 {
		maxTPS = rows[0].tps
	}

	row := panelsRow(stack, gap,
		panelSpec{"◈ MODELS · load · " + span, colCyan, lw, m.modelsBody(lw, 8)},
		panelSpec{"◈ OUTPUT SPEED · " + span, colCyan, rw, speedLanes(rows, maxTPS, rw)},
	)

	return vstack(engines, row, mix)
}
