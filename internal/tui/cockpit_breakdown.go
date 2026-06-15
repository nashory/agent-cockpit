package tui

import "fmt"

// breakdownView decomposes usage by agent, model, and speed: the ENGINES share
// bar on top, the model load table and output-speed lanes in the middle, and a
// 100%-stacked area chart of model mix over time at the bottom. Zoom (enter) any
// of them for full detail.
func (m Model) breakdownView(width int) string {
	prices := m.reportOptions.Pricing
	span := m.dataSpanLabel()
	mixH := m.breakdownMixHeight()
	modelRows := m.panelListRows(8, 12)
	engines := panel("◈ ENGINES · share · "+span, colCyan, width, m.enginesBar(m.events, prices, width))
	mix := panel(fmt.Sprintf("◈ MODEL MIX · %dd share", modelStackDays(width)), colCyan, width, m.modelStack(width, mixH))

	if m.compact {
		return vstack(engines, panel("◈ MODELS · load · "+span, colCyan, width, m.modelsBody(width, m.panelListRows(5, 8))), mix)
	}

	gap := 1
	cw, stack := gridWidths(width, gap, 2, 40)
	lw, rw := cw, cw
	if stack {
		lw, rw = width, width
	}
	rows := speedRows(m.events)
	speedRowsLimit := m.panelListRows(8, 12)
	if len(rows) > speedRowsLimit {
		rows = rows[:speedRowsLimit]
	}
	maxTPS := 1.0
	if len(rows) > 0 && rows[0].tps > 0 {
		maxTPS = rows[0].tps
	}

	row := panelsRow(stack, gap,
		panelSpec{"◈ MODELS · load · " + span, colCyan, lw, m.modelsBody(lw, modelRows)},
		panelSpec{"◈ OUTPUT SPEED · " + span, colCyan, rw, speedLanes(rows, maxTPS, rw)},
	)

	return vstack(engines, row, mix)
}

func (m Model) breakdownMixHeight() int {
	if m.height <= 0 {
		return 12
	}
	h := m.height - 29
	if h < 8 {
		return 8
	}
	if h > 24 {
		return 24
	}
	return h
}
