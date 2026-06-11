package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/usage"
	"github.com/spf13/cobra"
)

func serveCommand(opts *options) *cobra.Command {
	addr := "127.0.0.1:8765"
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve localhost JSON usage endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv := &http.Server{
				Addr:              addr,
				Handler:           serveMux(opts),
				ReadHeaderTimeout: 5 * time.Second,
			}
			fmt.Fprintf(os.Stderr, "Serving agent-cockpit on http://%s\n", addr)
			err := srv.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		},
	}
	cmd.Flags().StringVar(&addr, "addr", addr, "listen address")
	return cmd
}

func serveMux(opts *options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeServeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /api/summary", serveUsageHandler(opts, "summary"))
	mux.HandleFunc("GET /api/daily", serveUsageHandler(opts, "daily"))
	mux.HandleFunc("GET /api/blocks", serveUsageHandler(opts, "blocks"))
	mux.HandleFunc("GET /api/sessions", serveUsageHandler(opts, "sessions"))
	mux.HandleFunc("GET /api/statusline", serveStatuslineHandler(opts))
	return mux
}

func serveUsageHandler(opts *options, reportName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, cfg, loc, timezone, err := serveLoad(r, opts)
		if err != nil {
			serveError(w, err)
			return
		}
		ctx := buildUsageJSONContext(opts, timezone)
		ctx.Report = reportName
		ctx.Loc = loc
		w.Header().Set("Content-Type", "application/json")
		if err := writeUsageJSON(w, events, cfg, time.Now().In(loc), ctx); err != nil {
			fmt.Fprintf(os.Stderr, "serve %s: %v\n", r.URL.Path, err)
		}
	}
}

func serveStatuslineHandler(opts *options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, cfg, _, _, err := serveLoad(r, opts)
		if err != nil {
			serveError(w, err)
			return
		}
		statusOpts := *opts
		statusOpts.json = true
		statusOpts.statusline = nil
		w.Header().Set("Content-Type", "application/json")
		writeStatusline(w, events, reportOptions(cfg, &statusOpts), &statusOpts)
	}
}

func serveLoad(r *http.Request, opts *options) ([]usage.Event, config.Config, *time.Location, string, error) {
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return nil, config.Config{}, nil, "", err
	}
	loc, timezone, err := locationFor(opts, cfg)
	if err != nil {
		return nil, config.Config{}, nil, "", err
	}
	events, cfg, err := load(r.Context(), opts)
	if err != nil {
		return nil, config.Config{}, nil, "", err
	}
	return events, cfg, loc, timezone, nil
}

func serveError(w http.ResponseWriter, err error) {
	fmt.Fprintf(os.Stderr, "serve request: %v\n", err)
	writeServeJSON(w, http.StatusInternalServerError, map[string]any{
		"ok":    false,
		"error": err.Error(),
	})
}

func writeServeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
