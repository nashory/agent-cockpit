package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	if m.view != blocks {
		t.Fatalf("shift+tab from overview should wrap to blocks, got %d", m.view)
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
	if m.view != blocks || m.scroll != 0 {
		t.Fatalf("expected blocks/scroll 0, got view=%d scroll=%d", m.view, m.scroll)
	}
	if m.maxScroll() <= 0 {
		t.Fatalf("sample data should overflow the blocks table, maxScroll=%d", m.maxScroll())
	}
	// Down scrolls, up clamps at 0.
	m = upd(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.scroll != 1 {
		t.Fatalf("down should scroll to 1, got %d", m.scroll)
	}
	m = upd(m, tea.KeyMsg{Type: tea.KeyUp})
	m = upd(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.scroll != 0 {
		t.Fatalf("up past top should clamp to 0, got %d", m.scroll)
	}
	// end jumps to the bottom, capped at maxScroll.
	m = upd(m, runes("G"))
	if m.scroll != m.maxScroll() {
		t.Fatalf("G should jump to maxScroll %d, got %d", m.maxScroll(), m.scroll)
	}
	// Switching tabs resets the scroll offset.
	m = upd(m, runes("1"))
	if m.scroll != 0 {
		t.Fatalf("tab switch should reset scroll, got %d", m.scroll)
	}
}
