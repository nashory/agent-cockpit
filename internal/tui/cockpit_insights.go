package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// meter renders "LABEL  ███░░ 78%" — a labeled ratio gauge for a panel of the
// given inner text width.
func meter(label string, ratio float64, innerW int) string {
	g := innerW - 10 - 7 // label(10) + " 000%"(6) + 1 safety
	if g < 6 {
		g = 6
	}
	return labelStyle.Render(fmt.Sprintf("%-10s", label)) + gauge(ratio, g) + fmt.Sprintf(" %4.0f%%", ratio*100)
}

func (m Model) efficiencyBody(innerW int) string {
	ins, cur := m.ins, m.currency()
	return lipgloss.JoinVertical(lipgloss.Left,
		meter("CACHE HIT", ins.CacheHitRate, innerW),
		meter("REASONING", ins.ReasoningShare, innerW),
		"",
		kv("SAVED", fmt.Sprintf("%.2f %s", ins.CacheSavingsUSD, cur), colGreen),
		kv("IN:OUT", fmt.Sprintf("%.1f : 1", ins.InputOutputRatio), colText),
		kv("$/M out", fmt.Sprintf("%.2f %s", ins.EffectiveRateUSD, cur), colAmber),
	)
}

func (m Model) economicsBody(innerW int) string {
	ins, cur := m.ins, m.currency()
	return lipgloss.JoinVertical(lipgloss.Left,
		meter("PREMIUM", ins.PremiumCostShare, innerW),
		"",
		kv("TOTAL", fmt.Sprintf("%.2f %s", ins.TotalCostUSD, cur), colGreen),
		kv("PER DAY", fmt.Sprintf("%.2f %s", ins.CostPerActiveDay, cur), colAmber),
		kv("PROJ 30d", fmt.Sprintf("%.2f %s", ins.ProjectedMonthlyUSD, cur), colAmber),
		kv("PER SESS", fmt.Sprintf("%.3f %s", ins.AvgCostPerSession, cur), colText),
	)
}

func (m Model) cadenceBody() string {
	ins := m.ins
	return lipgloss.JoinVertical(lipgloss.Left,
		kv("SESSIONS", compact(int64(ins.Sessions)), colCyan),
		kv("ACTIVE", fmt.Sprintf("%d / %dd", ins.ActiveDays, ins.SpanDays), colText),
		kv("SESS/DAY", fmt.Sprintf("%.1f", ins.SessionsPerActiveDay), colText),
		kv("TOK/SESS", compact(int64(ins.AvgTokensPerSession)), colText),
		kv("PROJECTS", compact(int64(ins.Projects)), colGreen),
		kv("MODELS", compact(int64(ins.Models)), colGreen),
	)
}
