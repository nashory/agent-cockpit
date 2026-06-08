package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nashory/agent-cockpit/internal/report"
	"github.com/nashory/agent-cockpit/internal/usage"
)

func sampleEvents() []usage.Event {
	now := time.Now()
	projects := []string{"agent-cockpit", "coderxdox", "ttygg", "youtube-tools"}
	var evs []usage.Event
	add := func(d, hour int, src, model, proj, sess string, in, out, cr int64) {
		ts := now.AddDate(0, 0, -d)
		ts = time.Date(ts.Year(), ts.Month(), ts.Day(), hour, d%60, 0, 0, ts.Location())
		evs = append(evs, usage.Event{
			Source: src, Model: model, Project: proj, SessionID: sess,
			Input: in, Output: out, CacheRead: cr,
			Timestamp: ts,
		})
	}
	for d := 0; d < 210; d++ {
		if d > 0 && d%11 == 0 {
			continue // idle days, so streaks/gaps show in the calendar
		}
		proj := projects[d%len(projects)]
		hour := 9 + d%9 // working hours, varying
		intensity := int64(1 + d%5)
		add(d, hour, "claude", "claude-opus-4-8", proj, fmt.Sprintf("c%d", d), (1000+int64(d*50))*intensity, 400, 8000)
		add(d, hour+1, "codex", "gpt-5-codex", proj, fmt.Sprintf("x%d", d), (3000+int64(d*120))*intensity, 600, 1200)
		if d%3 == 0 {
			add(d, 14, "gemini", "gemini-2.5-pro", proj, fmt.Sprintf("g%d", d), 500, 200, 0)
		}
	}
	return evs
}

// TestCockpitRender prints the dashboard so layout can be eyeballed:
//
//	go test ./internal/tui -run TestCockpitRender -v
func TestCockpitRender(t *testing.T) {
	tabs := []struct {
		name string
		v    view
	}{
		{"OVERVIEW", overview},
		{"BREAKDOWN", breakdown},
		{"TRENDS", trends},
		{"ACTIVITY", activity},
		{"DAILY", daily},
		{"BLOCKS", blocks},
	}
	for _, compactMode := range []bool{false, true} {
		for _, termW := range []int{140, 80} {
			for _, tc := range tabs {
				m := New(sampleEvents(), Options{
					Report:          report.Options{Currency: "USD"},
					RefreshInterval: 3 * time.Second,
				})
				m.width = termW
				m.height = 44
				m.view = tc.v
				m.compact = compactMode
				out := m.View()
				if out == "" {
					t.Fatalf("empty render for %s @ %d compact=%v", tc.name, termW, compactMode)
				}
				// No rendered line may exceed the terminal width (border-break check).
				for _, ln := range strings.Split(out, "\n") {
					if w := lipgloss.Width(ln); w > termW {
						t.Errorf("%s @ %d compact=%v: line width %d exceeds %d:\n%s", tc.name, termW, compactMode, w, termW, ln)
						break
					}
				}
				if termW == 140 {
					mode := "expert"
					if compactMode {
						mode = "compact"
					}
					fmt.Printf("\n===== %s · %s (width=140) =====\n%s\n", tc.name, mode, out)
				}
			}
		}
	}
}

// TestFocusTargetsMatchBody guards against the FOCUS bar drifting out of sync
// with the panels a tab actually draws: every focus chip must correspond to a
// "◈ <title>" instrument header in the rendered body, in both expert and
// compact layouts. (Compact renders fewer panels, so the target list must
// shrink to match.)
func TestFocusTargetsMatchBody(t *testing.T) {
	for _, compactMode := range []bool{false, true} {
		for _, v := range []view{overview, breakdown, trends, activity, daily, blocks} {
			m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
			m.width = 140
			m.height = 44
			m.view = v
			m.compact = compactMode
			body := stripANSI(m.View())
			for _, tgt := range m.zoomTargets() {
				header := "◈ " + tgt.title
				if !strings.Contains(body, header) {
					t.Errorf("view=%d compact=%v: FOCUS chip %q has no matching %q panel in the body",
						v, compactMode, tgt.title, header)
				}
			}
		}
	}
}

// stripANSI removes SGR escape sequences so rendered text can be matched.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++ // skip the trailing 'm'
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func TestZoomRender(t *testing.T) {
	views := []view{overview, breakdown, trends, activity, daily, blocks}
	for _, v := range views {
		for _, w := range []int{140, 80} {
			m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
			m.width = w
			m.height = 44
			m.view = v
			m.zoomed = true
			for f := 0; f < len(m.zoomTargets()); f++ {
				m.focus = f
				out := m.View()
				if out == "" {
					t.Fatalf("empty zoom render view=%d focus=%d", v, f)
				}
				for _, ln := range strings.Split(out, "\n") {
					if lw := lipgloss.Width(ln); lw > w {
						t.Errorf("zoom view=%d focus=%d @ %d: line width %d > %d", v, f, w, lw, w)
						break
					}
				}
			}
		}
	}
}

func TestCalendarCursor(t *testing.T) {
	m := New(sampleEvents(), Options{Report: report.Options{Currency: "USD"}})
	m.view = activity
	m.focus = 0     // CALENDAR is the first widget on the Activity tab
	m.zoomed = true // cursor is active only when zoomed into the calendar

	step := func(k tea.KeyType) {
		nm, _ := m.Update(tea.KeyMsg{Type: k})
		m = nm.(Model)
	}

	step(tea.KeyLeft) // older by a week
	if m.calCursor != 7 {
		t.Fatalf("left should move +7, got %d", m.calCursor)
	}
	step(tea.KeyRight)
	step(tea.KeyRight) // back to 0, then clamp
	if m.calCursor != 0 {
		t.Fatalf("cursor should clamp at 0, got %d", m.calCursor)
	}
	step(tea.KeyUp) // older by a day
	if m.calCursor != 1 {
		t.Fatalf("up should move +1, got %d", m.calCursor)
	}

	// Off the calendar widget, arrows move widget focus and must not move the cursor.
	m.view = overview
	prev := m.calCursor
	step(tea.KeyLeft)
	if m.calCursor != prev {
		t.Fatalf("cursor moved while not zoomed on the calendar")
	}
}

func TestTruncateUTF8(t *testing.T) {
	// Korean project name: 9 runes / 27 bytes. Byte-slicing would corrupt it.
	s := "컨텍스트엔지니어링"
	got := truncate(s, 5)
	if !utf8.ValidString(got) {
		t.Fatalf("truncate produced invalid UTF-8: %q", got)
	}
	if r := []rune(got); len(r) != 5 {
		t.Fatalf("expected 5 runes, got %d (%q)", len(r), got)
	}
	if truncate("abc", 10) != "abc" {
		t.Fatalf("short string should be unchanged")
	}
}
