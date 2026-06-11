package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// acBin is the freshly built binary exercised by the smoke tests.
var acBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cockpit-smoke")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mktemp:", err)
		os.Exit(1)
	}
	acBin = filepath.Join(dir, "cockpit")
	if runtime.GOOS == "windows" {
		acBin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", acBin, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// tempConfig writes a config whose three sources point at temp dirs (so smoke
// tests never read real ~/.claude logs) with one recent Claude event.
func tempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	claudeRoot := filepath.Join(dir, "claude")
	empty := filepath.Join(dir, "empty")
	proj := filepath.Join(claudeRoot, "proj")
	for _, d := range []string{proj, empty} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ts := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	line := fmt.Sprintf(`{"type":"assistant","timestamp":%q,"sessionId":"s1","message":{"model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":50}}}`, ts)
	if err := os.WriteFile(filepath.Join(proj, "a.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// TOML literal (single-quoted) strings keep Windows backslash paths intact.
	body := fmt.Sprintf("[paths]\nclaude = ['%s']\ncodex = ['%s']\ngemini = ['%s']\n", claudeRoot, empty, empty)
	cfg := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func runAC(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command(acBin, args...).CombinedOutput()
	return string(out), err
}

func TestSmokeTodayJSON(t *testing.T) {
	cfg := tempConfig(t)
	out, err := runAC(t, "--config", cfg, "today", "--json")
	if err != nil {
		t.Fatalf("today --json failed: %v\n%s", err, out)
	}
	var doc struct {
		Totals struct {
			Events int   `json:"events"`
			Total  int64 `json:"total_tokens"`
		} `json:"totals"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if doc.Totals.Events != 1 || doc.Totals.Total != 150 {
		t.Fatalf("totals = %+v, want events 1 / total 150", doc.Totals)
	}
}

func TestSmokeReportSubcommands(t *testing.T) {
	cfg := tempConfig(t)
	for _, sub := range []string{"today", "weekly", "monthly", "agents", "sessions", "trends", "speed", "statusline"} {
		out, err := runAC(t, "--config", cfg, sub)
		if err != nil {
			t.Fatalf("%q failed: %v\n%s", sub, err, out)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("%q produced no output", sub)
		}
	}
}

func TestSmokeStatuslineContent(t *testing.T) {
	cfg := tempConfig(t)
	out, err := runAC(t, "--config", cfg, "statusline")
	if err != nil {
		t.Fatalf("statusline failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "tokens") || !strings.Contains(out, "cost") {
		t.Fatalf("statusline missing expected fields: %s", out)
	}
}

func TestSmokeStatuslineJSON(t *testing.T) {
	cfg := tempConfig(t)
	out, err := runAC(t, "--config", cfg, "--json", "statusline")
	if err != nil {
		t.Fatalf("statusline --json failed: %v\n%s", err, out)
	}
	var doc struct {
		Totals struct {
			Total int64 `json:"total_tokens"`
		} `json:"totals"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid statusline JSON: %v\n%s", err, out)
	}
	if doc.Totals.Total != 150 {
		t.Fatalf("statusline total = %d, want 150", doc.Totals.Total)
	}
}

func TestSmokeExportCSV(t *testing.T) {
	cfg := tempConfig(t)
	out, err := runAC(t, "--config", cfg, "export", "--group", "daily")
	if err != nil {
		t.Fatalf("export failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "date,events,input_tokens") || !strings.Contains(out, "150") {
		t.Fatalf("export output unexpected: %s", out)
	}
}

func TestSmokePricingStatus(t *testing.T) {
	cfg := tempConfig(t)
	out, err := runAC(t, "--config", cfg, "pricing", "status")
	if err != nil {
		t.Fatalf("pricing status failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Vendored pricing models") || !strings.Contains(out, "claude-opus") {
		t.Fatalf("pricing status unexpected: %s", out)
	}
}

func TestSmokeConfigSchemaAndValidate(t *testing.T) {
	cfg := tempConfig(t)
	out, err := runAC(t, "config", "schema")
	if err != nil {
		t.Fatalf("config schema failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"properties"`) || !strings.Contains(out, `"paths"`) {
		t.Fatalf("config schema output unexpected: %s", out)
	}
	out, err = runAC(t, "--config", cfg, "config", "validate")
	if err != nil {
		t.Fatalf("config validate failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "config ok:") {
		t.Fatalf("config validate output unexpected: %s", out)
	}
}

func TestSmokeDoctor(t *testing.T) {
	cfg := tempConfig(t)
	out, err := runAC(t, "--config", cfg, "doctor")
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Claude paths:") {
		t.Fatalf("doctor output unexpected: %s", out)
	}
}

func TestSmokeVersion(t *testing.T) {
	out, err := runAC(t, "--version")
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "version") {
		t.Fatalf("--version output unexpected: %s", out)
	}
}

func TestSmokeUnknownFlagFails(t *testing.T) {
	if _, err := runAC(t, "--definitely-not-a-flag"); err == nil {
		t.Fatal("expected a non-zero exit for an unknown flag")
	}
}
