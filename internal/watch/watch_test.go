package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestIsLogFile(t *testing.T) {
	cases := map[string]bool{
		"/a/b/session.jsonl":                   true,
		"/a/b/archived_sessions/x.jsonl":       true,
		"/g/tmp/h/chats/session-1.json":        true,
		"/codebuff/chats/a/chat-messages.json": true,
		"/opencode/opencode.db":                true,
		"/kilo/kilo.db":                        true,
		"/goose/sessions.db":                   true,
		"/g/tmp/h/chats/notes.json":            false, // json but not a session file
		"/a/b/config.toml":                     false,
		"/a/b/session-1.jsonl.tmp":             false,
		"/a/b/chat-messages.json.tmp":          false,
	}
	for path, want := range cases {
		if got := IsLogFile(path); got != want {
			t.Errorf("IsLogFile(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestDirsUnder(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "a"))
	mustMkdir(t, filepath.Join(root, "a", "b"))
	mustMkdir(t, filepath.Join(root, "c"))
	mustWrite(t, filepath.Join(root, "a", "f.jsonl"), "x")

	dirs := dirsUnder(root)
	want := map[string]bool{
		root:                          true,
		filepath.Join(root, "a"):      true,
		filepath.Join(root, "a", "b"): true,
		filepath.Join(root, "c"):      true,
	}
	if len(dirs) != len(want) {
		t.Fatalf("got %d dirs %v, want %d", len(dirs), dirs, len(want))
	}
	for _, d := range dirs {
		if !want[d] {
			t.Errorf("unexpected dir %q (files must not be included)", d)
		}
	}
}

func TestDirsUnderFileRootReturnsParent(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "kilo.db")
	mustWrite(t, dbPath, "sqlite")

	dirs := dirsUnder(dbPath)
	if len(dirs) != 1 || dirs[0] != root {
		t.Fatalf("dirsUnder(file) = %v, want [%q]", dirs, root)
	}
}

func TestRelevant(t *testing.T) {
	cases := []struct {
		ev   fsnotify.Event
		want bool
	}{
		{fsnotify.Event{Name: "/a/x.jsonl", Op: fsnotify.Write}, true},
		{fsnotify.Event{Name: "/a/x.jsonl", Op: fsnotify.Create}, true},
		{fsnotify.Event{Name: "/a/x.jsonl", Op: fsnotify.Chmod}, false}, // chmod is not a content change
		{fsnotify.Event{Name: "/a/x.txt", Op: fsnotify.Write}, false},   // not a log file
	}
	for _, c := range cases {
		if got := relevant(c.ev, IsLogFile); got != c.want {
			t.Errorf("relevant(%v) = %v, want %v", c.ev, got, c.want)
		}
	}
}

func TestDebounceCoalesces(t *testing.T) {
	in := make(chan struct{}, 16)
	out := make(chan struct{}, 16)
	done := make(chan struct{})
	defer close(done)
	go debounce(20*time.Millisecond, in, out, done)

	// A burst of 5 pulses should collapse into exactly one signal.
	for i := 0; i < 5; i++ {
		in <- struct{}{}
	}
	if !recv(out, 500*time.Millisecond) {
		t.Fatal("expected one debounced signal from a burst")
	}
	if recv(out, 60*time.Millisecond) {
		t.Fatal("a single burst must not emit a second signal")
	}

	// A later pulse, after the quiet window, emits again.
	in <- struct{}{}
	if !recv(out, 500*time.Millisecond) {
		t.Fatal("expected a signal for the second burst")
	}
}

func TestWatcherEmitsOnWrite(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "proj")
	mustMkdir(t, sub)

	w, err := New([]string{root}, IsLogFile, 20*time.Millisecond)
	if err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}
	defer w.Close()

	mustWrite(t, filepath.Join(sub, "a.jsonl"), "line\n")
	if !recv(w.Events(), 3*time.Second) {
		t.Fatal("expected a refresh signal after writing a .jsonl file")
	}
}

func TestWatcherEmitsForFileRoot(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "kilo.db")
	mustWrite(t, dbPath, "one")

	w, err := New([]string{dbPath}, IsLogFile, 20*time.Millisecond)
	if err != nil {
		t.Skipf("fsnotify unavailable: %v", err)
	}
	defer w.Close()

	mustWrite(t, dbPath, "two")
	if !recv(w.Events(), 3*time.Second) {
		t.Fatal("expected a refresh signal after writing a watched file root")
	}
}

func recv(ch <-chan struct{}, d time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(d):
		return false
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
