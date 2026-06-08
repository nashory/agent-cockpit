package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
		{"AGENTS", agents},
		{"MODELS", models},
		{"TRENDS", trends},
		{"SPEED", speed},
		{"INSIGHTS", insights},
		{"ACTIVITY", activity},
		{"CALENDAR", calendar},
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
