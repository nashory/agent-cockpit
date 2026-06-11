package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeMux(t *testing.T) {
	cfgPath := serveTestConfig(t)
	mux := serveMux(&options{configPath: cfgPath, days: 30})

	health := httptest.NewRecorder()
	mux.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"ok":true`) {
		t.Fatalf("health response = %d %s", health.Code, health.Body.String())
	}

	for _, tc := range []struct {
		path   string
		report string
	}{
		{path: "/api/summary", report: "summary"},
		{path: "/api/daily", report: "daily"},
		{path: "/api/blocks", report: "blocks"},
		{path: "/api/sessions", report: "sessions"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("%s status = %d body=%s", tc.path, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), `"report": "`+tc.report+`"`) {
				t.Fatalf("%s missing report %q:\n%s", tc.path, tc.report, w.Body.String())
			}
		})
	}

	statusline := httptest.NewRecorder()
	mux.ServeHTTP(statusline, httptest.NewRequest(http.MethodGet, "/api/statusline", nil))
	if statusline.Code != http.StatusOK || !strings.Contains(statusline.Body.String(), `"schema_version": "1"`) {
		t.Fatalf("statusline response = %d %s", statusline.Code, statusline.Body.String())
	}
}

func serveTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, "claude")
	emptyDir := filepath.Join(dir, "empty")
	if err := os.MkdirAll(filepath.Join(claudeDir, "proj"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(claudeDir, "proj", "a.jsonl"),
		`{"type":"assistant","timestamp":"2026-06-11T08:00:00Z","sessionId":"s1","cwd":"/repo","message":{"model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":50}}}`)
	cfgPath := filepath.Join(dir, "config.toml")
	mustWrite(t, cfgPath, "[paths]\n"+
		"claude = ['"+claudeDir+"']\n"+
		"codex = ['"+emptyDir+"']\n"+
		"gemini = ['"+emptyDir+"']\n")
	return cfgPath
}
