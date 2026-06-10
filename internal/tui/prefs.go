package tui

import (
	"encoding/json"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nashory/agent-cockpit/internal/config"
)

// uiPrefs is the small set of view preferences that survive across runs, the way
// the Claude-Code usage monitor remembers your last_used settings. It is stored
// next to the config file so a fresh launch reopens in the layout you left.
type uiPrefs struct {
	Compact    bool `json:"compact"`
	WindowDays int  `json:"window_days"`
	SortMode   int  `json:"sort_mode"`
	PeriodMode int  `json:"period_mode"`
}

// prefsPath puts ui.json next to the config file (same agent-cockpit dir), so a
// user finds both preferences and config in one place.
func prefsPath() string {
	return filepath.Join(filepath.Dir(config.ConfigPath()), "ui.json")
}

// loadPrefs reads saved preferences, returning sane defaults when none exist or
// the file is unreadable/corrupt. WindowDays is constrained to the values the w
// key cycles, and SortMode to the three table sorts.
func loadPrefs() uiPrefs {
	p := uiPrefs{WindowDays: 30}
	path := prefsPath()
	if path == "" {
		return p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return p
	}
	var loaded uiPrefs
	if err := json.Unmarshal(data, &loaded); err != nil {
		return p
	}
	switch loaded.WindowDays {
	case 7, 30, 90:
		p.WindowDays = loaded.WindowDays
	}
	if loaded.SortMode >= 0 && loaded.SortMode <= 2 {
		p.SortMode = loaded.SortMode
	}
	if loaded.PeriodMode >= 0 && loaded.PeriodMode <= 2 {
		p.PeriodMode = loaded.PeriodMode
	}
	p.Compact = loaded.Compact
	return p
}

// savePrefs persists preferences best-effort; any error (no config dir, no write
// permission) is ignored so a read-only home never breaks the dashboard.
func savePrefs(p uiPrefs) {
	path := prefsPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

// prefs snapshots the model's persistable view preferences.
func (m Model) prefs() uiPrefs {
	return uiPrefs{Compact: m.compact, WindowDays: m.windowDays, SortMode: m.sortMode, PeriodMode: m.periodMode}
}

// savePrefsCmd writes preferences off the render path as a Bubble Tea command, so
// a key press that changes a preference never blocks on disk I/O.
func savePrefsCmd(p uiPrefs) tea.Cmd {
	return func() tea.Msg {
		savePrefs(p)
		return nil
	}
}
