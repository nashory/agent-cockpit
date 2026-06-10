package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/report"
)

func upd(m Model, msg tea.Msg) Model {
	nm, _ := m.Update(msg)
	return nm.(Model)
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestUpdateKeyHandling(t *testing.T) {
	m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
	m.width, m.height = 140, 44

	// Number keys jump tabs.
	m = upd(m, runes("3"))
	if m.view != trends {
		t.Fatalf("'3' should select trends, got %d", m.view)
	}

	// Overview has several focusable widgets; arrows move focus and wrap.
	m = upd(m, runes("1"))
	if m.view != overview || m.focus != 0 {
		t.Fatalf("'1' should reset to overview/focus 0, got view=%d focus=%d", m.view, m.focus)
	}
	n := len(m.zoomTargets())
	if n < 2 {
		t.Fatalf("overview should expose multiple widgets, got %d", n)
	}
	m = upd(m, tea.KeyMsg{Type: tea.KeyRight})
	if m.focus != 1 {
		t.Fatalf("right should move focus to 1, got %d", m.focus)
	}
	m = upd(m, tea.KeyMsg{Type: tea.KeyLeft})
	m = upd(m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.focus != n-1 {
		t.Fatalf("left past 0 should wrap to %d, got %d", n-1, m.focus)
	}

	// Enter zooms, esc returns.
	m = upd(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.zoomed {
		t.Fatal("enter should zoom")
	}
	m = upd(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.zoomed {
		t.Fatal("esc should exit zoom")
	}

	// 'e' toggles compact mode.
	if m.compact {
		t.Fatal("expected expert mode by default")
	}
	m = upd(m, runes("e"))
	if !m.compact {
		t.Fatal("'e' should toggle compact mode")
	}

	// Tab advances the view and resets focus/zoom.
	m.focus, m.zoomed = 2, true
	m = upd(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.view != breakdown || m.focus != 0 || m.zoomed {
		t.Fatalf("tab should advance view and reset focus/zoom, got view=%d focus=%d zoomed=%v", m.view, m.focus, m.zoomed)
	}

	// shift+tab wraps backwards from overview to the last tab.
	m = upd(m, runes("1"))
	m = upd(m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.view != sessions {
		t.Fatalf("shift+tab from overview should wrap to sessions, got %d", m.view)
	}
}

func TestUpdateQuits(t *testing.T) {
	m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
	if _, cmd := m.Update(runes("q")); cmd == nil {
		t.Fatal("'q' should return a quit command")
	}
}

func TestTableScroll(t *testing.T) {
	m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
	m.width, m.height = 140, 44

	m = upd(m, runes("6")) // Blocks tab
	if m.view != blocks || m.blkSel != 0 {
		t.Fatalf("expected blocks/blkSel 0, got view=%d blkSel=%d", m.view, m.blkSel)
	}
	if m.tableTotal() <= 1 {
		t.Fatalf("sample data should produce many blocks, got %d", m.tableTotal())
	}
	// Down moves the cursor, up clamps at 0.
	m = upd(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.blkSel != 1 {
		t.Fatalf("down should move cursor to 1, got %d", m.blkSel)
	}
	m = upd(m, tea.KeyMsg{Type: tea.KeyUp})
	m = upd(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.blkSel != 0 {
		t.Fatalf("up past top should clamp to 0, got %d", m.blkSel)
	}
	// end jumps the cursor to the last row and scroll follows.
	m = upd(m, runes("G"))
	if m.blkSel != m.tableTotal()-1 {
		t.Fatalf("G should move cursor to last row %d, got %d", m.tableTotal()-1, m.blkSel)
	}
	// Enter opens the window popup; esc closes it.
	m = upd(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.blkPopup {
		t.Fatal("enter should open the block popup")
	}
	if out := stripANSI(m.View()); !strings.Contains(out, "WINDOW ·") || !strings.Contains(out, "MODEL") {
		t.Fatalf("block popup should show per-model breakdown:\n%s", out)
	}
	m = upd(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.blkPopup {
		t.Fatal("esc should close the block popup")
	}
	// Switching tabs resets cursor and scroll.
	m = upd(m, runes("1"))
	if m.scroll != 0 || m.blkSel != 0 {
		t.Fatalf("tab switch should reset, got scroll=%d blkSel=%d", m.scroll, m.blkSel)
	}
}

func TestDailyPopup(t *testing.T) {
	m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
	m.width, m.height = 140, 44
	m = upd(m, runes("5")) // Daily tab
	if m.view != daily {
		t.Fatalf("expected daily, got %d", m.view)
	}
	// Cursor moves down.
	m = upd(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.daySel != 1 {
		t.Fatalf("down should move daySel to 1, got %d", m.daySel)
	}
	// Enter opens the per-model popup.
	m = upd(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.dayPopup {
		t.Fatal("enter should open the day popup")
	}
	out := stripANSI(m.View())
	if !strings.Contains(out, "DAILY ·") || !strings.Contains(out, "MODEL") {
		t.Fatalf("popup should show the day's per-model breakdown:\n%s", out)
	}
	// Esc closes it.
	m = upd(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.dayPopup {
		t.Fatal("esc should close the day popup")
	}
}

func TestSessionsPopup(t *testing.T) {
	m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
	m.width, m.height = 140, 44
	m = upd(m, runes("7")) // Sessions tab
	if m.view != sessions {
		t.Fatalf("'7' should select sessions, got %d", m.view)
	}
	if m.tableTotal() <= 1 {
		t.Fatalf("sample data should produce many sessions, got %d", m.tableTotal())
	}
	// Cursor moves down.
	m = upd(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.sessSel != 1 {
		t.Fatalf("down should move sessSel to 1, got %d", m.sessSel)
	}
	// Enter opens the per-model popup.
	m = upd(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.sessPopup {
		t.Fatal("enter should open the session popup")
	}
	out := stripANSI(m.View())
	if !strings.Contains(out, "SESSION ·") || !strings.Contains(out, "MODEL") {
		t.Fatalf("popup should show the session's per-model breakdown:\n%s", out)
	}
	// Esc closes it.
	m = upd(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.sessPopup {
		t.Fatal("esc should close the session popup")
	}
}

func TestHelpOverlay(t *testing.T) {
	m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
	m.width, m.height = 140, 44
	m = upd(m, runes("?"))
	if !m.showHelp {
		t.Fatal("? should open help")
	}
	out := stripANSI(m.View())
	if !strings.Contains(out, "KEYBOARD") || !strings.Contains(out, "per-model breakdown") {
		t.Fatalf("help should list keys:\n%s", out)
	}
	// No line exceeds the terminal width.
	for _, ln := range strings.Split(m.View(), "\n") {
		if lipgloss.Width(ln) > 140 {
			t.Fatalf("help line too wide: %q", ln)
		}
	}
	m = upd(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.showHelp {
		t.Fatal("esc should close help")
	}
}

func TestWindowCycle(t *testing.T) {
	m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
	m.width, m.height = 140, 44
	if m.windowDays != 30 {
		t.Fatalf("default window should be 30, got %d", m.windowDays)
	}
	m = upd(m, runes("w"))
	if m.windowDays != 90 {
		t.Fatalf("w should go 30->90, got %d", m.windowDays)
	}
	m = upd(m, runes("w"))
	if m.windowDays != 7 {
		t.Fatalf("w should go 90->7, got %d", m.windowDays)
	}
	// trendSel is reclamped into the new window.
	if m.trendSel != 6 {
		t.Fatalf("trendSel should reset to window-1 (6), got %d", m.trendSel)
	}
}

func TestSortCycle(t *testing.T) {
	m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
	m.width, m.height = 140, 44
	m = upd(m, runes("5")) // daily
	m = upd(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.daySel == 0 {
		t.Fatal("cursor should have moved")
	}
	m = upd(m, runes("s"))
	if m.sortMode != 1 || m.daySel != 0 {
		t.Fatalf("s should set sort=1 and reset cursor, got sort=%d daySel=%d", m.sortMode, m.daySel)
	}
	// Sorted by tokens desc: first row >= second row.
	rows := m.sortedLedger()
	if len(rows) >= 2 && rows[0].totals.Total < rows[1].totals.Total {
		t.Fatalf("tokens sort not descending: %d < %d", rows[0].totals.Total, rows[1].totals.Total)
	}
	if out := stripANSI(m.View()); !strings.Contains(out, "sort tokens") {
		t.Fatalf("title should show sort mode:\n%s", out[:200])
	}
}
