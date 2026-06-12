// Package watch turns local log-directory file-system changes into debounced
// refresh signals, so the TUI can update on write instead of polling.
package watch

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDebounce coalesces a burst of writes into a single refresh.
const DefaultDebounce = 300 * time.Millisecond

// IsLogFile reports whether path is an agent usage file worth refreshing on.
func IsLogFile(path string) bool {
	base := filepath.Base(path)
	switch {
	case strings.HasSuffix(path, ".jsonl"):
		return true
	case base == "chat-messages.json":
		return true
	case strings.HasSuffix(path, ".json") && strings.HasPrefix(base, "session-"):
		return true
	case strings.HasSuffix(path, ".db"):
		return true
	default:
		return false
	}
}

// dirsUnder returns root and all existing subdirectories (best-effort). If root
// is a file path, it returns the parent directory so changes to that file can be
// observed.
func dirsUnder(root string) []string {
	if st, err := os.Stat(root); err == nil && !st.IsDir() {
		return []string{filepath.Dir(root)}
	}
	var dirs []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	return dirs
}

// relevant reports whether an fs event should trigger a refresh.
func relevant(ev fsnotify.Event, match func(string) bool) bool {
	if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
		return false
	}
	return match(ev.Name)
}

// Watcher recursively watches log roots and emits a debounced signal on Events
// whenever a matching file changes. New subdirectories are watched as they
// appear, so fresh project folders are picked up automatically.
type Watcher struct {
	fsw    *fsnotify.Watcher
	match  func(string) bool
	events chan struct{}
	done   chan struct{}
}

// New starts a watcher over roots. match defaults to IsLogFile. It returns an
// error if the OS watcher cannot be created; callers should fall back to
// polling in that case.
func New(roots []string, match func(string) bool, debounceDelay time.Duration) (*Watcher, error) {
	if match == nil {
		match = IsLogFile
	}
	if debounceDelay <= 0 {
		debounceDelay = DefaultDebounce
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fsw:    fsw,
		match:  match,
		events: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			continue
		}
		for _, dir := range dirsUnder(root) {
			_ = fsw.Add(dir)
		}
	}
	pulses := make(chan struct{}, 1)
	go w.watchLoop(pulses)
	go debounce(debounceDelay, pulses, w.events, w.done)
	return w, nil
}

// Events delivers a signal each time matching files settle after changes.
func (w *Watcher) Events() <-chan struct{} { return w.events }

// Close stops the watcher and releases OS resources.
func (w *Watcher) Close() error {
	close(w.done)
	return w.fsw.Close()
}

func (w *Watcher) watchLoop(pulses chan<- struct{}) {
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if ev.Op&fsnotify.Create != 0 {
				if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
					for _, dir := range dirsUnder(ev.Name) {
						_ = w.fsw.Add(dir)
					}
				}
			}
			if relevant(ev, w.match) {
				select {
				case pulses <- struct{}{}:
				default:
				}
			}
		case <-w.fsw.Errors:
			// Ignore watcher errors; a polling backstop covers any gaps.
		}
	}
}

// debounce coalesces pulses on in into single signals on out: it emits one
// signal once d elapses with no further input.
func debounce(d time.Duration, in <-chan struct{}, out chan<- struct{}, done <-chan struct{}) {
	if d <= 0 {
		d = DefaultDebounce
	}
	var timer *time.Timer
	var tc <-chan time.Time
	for {
		select {
		case <-done:
			return
		case <-in:
			if timer == nil {
				timer = time.NewTimer(d)
				tc = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(d)
			}
		case <-tc:
			tc = nil
			timer = nil
			select {
			case out <- struct{}{}:
			case <-done:
				return
			}
		}
	}
}
