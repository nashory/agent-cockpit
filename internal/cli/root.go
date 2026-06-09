package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/report"
	"github.com/nashory/agent-cockpit/internal/source"
	"github.com/nashory/agent-cockpit/internal/tui"
	"github.com/nashory/agent-cockpit/internal/usage"
	"github.com/nashory/agent-cockpit/internal/watch"
	"github.com/spf13/cobra"
)

type options struct {
	days       int
	since      string
	until      string
	sources    string
	project    string
	model      string
	configPath string
	refresh    string
	json       bool
	svgPath    string
}

var version = "dev"

func Execute() error {
	opts := &options{days: 30}
	root := &cobra.Command{
		Use:   "cockpit",
		Short: "Live usage, cost, and speed dashboards for coding agents",
		Long:  "cockpit is the command-line dashboard for local coding-agent logs: token usage, estimated cost, and trends without uploading your data.",
		// Print runtime errors once (from main), without dumping usage.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.json {
				events, cfg, err := load(cmd.Context(), opts)
				if err != nil {
					return err
				}
				return writeJSON(events, cfg)
			}
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			reload := func() ([]usage.Event, error) {
				events, _, err := load(cmd.Context(), opts)
				return events, err
			}
			_, err = tea.NewProgram(tui.New(nil, tuiOptions(cfg, opts, reload, 0)), tea.WithAltScreen()).Run()
			return err
		},
	}
	addFlags(root, opts)
	root.Version = version

	root.AddCommand(reportCommand("today", "Show today's static report", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Overview(w, "Today", events, ro)
	}, func() { opts.days = 1 }))
	root.AddCommand(reportCommand("weekly", "Show the last 7 days", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Overview(w, "Last 7 days", events, ro)
	}, func() { opts.days = 7 }))
	root.AddCommand(reportCommand("monthly", "Show the last 30 days", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Overview(w, "Last 30 days", events, ro)
	}, func() { opts.days = 30 }))
	root.AddCommand(reportCommand("agents", "Group usage by agent", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Buckets(w, "Agents", usage.GroupByWith(events, ro.Pricing, func(e usage.Event) string { return e.Source }), 0, ro)
	}, nil))
	root.AddCommand(reportCommand("sessions", "Show highest-usage sessions", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Sessions(w, events, 20, ro)
	}, nil))
	root.AddCommand(reportCommand("trends", "Show token and cost trends", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Trend(w, events, opts.days, ro)
	}, nil))
	root.AddCommand(reportCommand("speed", "Show observed output token speed by agent/model", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Speed(w, events, 20)
	}, nil))
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Print a usage summary, or write a shareable SVG card with --svg",
		RunE: func(cmd *cobra.Command, args []string) error {
			events, cfg, err := load(cmd.Context(), opts)
			if err != nil {
				return err
			}
			ro := reportOptions(cfg)
			if opts.svgPath != "" {
				f, err := os.Create(opts.svgPath)
				if err != nil {
					return err
				}
				defer f.Close()
				report.SVG(f, "Usage summary", events, ro)
				fmt.Fprintf(os.Stderr, "wrote %s\n", opts.svgPath)
				return nil
			}
			if opts.json {
				return writeJSON(events, cfg)
			}
			report.Overview(os.Stdout, "Usage summary", events, ro)
			return nil
		},
	}
	reportCmd.Flags().StringVar(&opts.svgPath, "svg", "", "write a shareable SVG card to this path")
	root.AddCommand(reportCmd)
	root.AddCommand(&cobra.Command{
		Use:   "live",
		Short: "Open the TUI and refresh local logs on an interval",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			interval := cfg.RefreshDuration()
			if opts.refresh != "" {
				parsed, err := time.ParseDuration(opts.refresh)
				if err != nil {
					return fmt.Errorf("parse --refresh: %w", err)
				}
				interval = parsed
			}
			reload := func() ([]usage.Event, error) {
				events, _, err := load(cmd.Context(), opts)
				return events, err
			}
			tuiOpts := tuiOptions(cfg, opts, reload, interval)
			// Refresh on file-system events when possible; the interval tick
			// above stays as a backstop if the watcher can't start.
			roots := append(append(append([]string{}, cfg.Paths.Claude...), cfg.Paths.Codex...), cfg.Paths.Gemini...)
			if w, werr := watch.New(roots, watch.IsLogFile, watch.DefaultDebounce); werr == nil {
				defer w.Close()
				tuiOpts.FSEvents = w.Events()
			}
			_, err = tea.NewProgram(tui.New(nil, tuiOpts), tea.WithAltScreen()).Run()
			return err
		},
	})
	root.AddCommand(reportCommand("statusline", "Print one-line usage for tmux/statusline integrations", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		t := usage.SummarizeWith(events, ro.Pricing)
		currency := ro.Currency
		if currency == "" {
			currency = "USD"
		}
		fmt.Fprintf(w, "tokens %d | cost %.2f %s | events %d\n", t.Total, t.CostUSD, currency, t.Events)
	}, nil))
	root.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Show detected log locations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			fmt.Printf("Config: %s\n", configPath(opts))
			fmt.Printf("Refresh: %s\n", cfg.RefreshDuration())
			fmt.Println("Claude paths:")
			for _, p := range cfg.Paths.Claude {
				printPath(p)
			}
			fmt.Println("Codex paths:")
			for _, p := range cfg.Paths.Codex {
				printPath(p)
			}
			fmt.Println("Gemini paths:")
			for _, p := range cfg.Paths.Gemini {
				printPath(p)
			}
			return nil
		},
	})
	root.AddCommand(configCommand(opts))

	return root.Execute()
}

func addFlags(cmd *cobra.Command, opts *options) {
	cmd.PersistentFlags().IntVar(&opts.days, "days", opts.days, "number of days to include")
	cmd.PersistentFlags().StringVar(&opts.since, "since", "", "start date, YYYY-MM-DD")
	cmd.PersistentFlags().StringVar(&opts.until, "until", "", "end date, YYYY-MM-DD")
	cmd.PersistentFlags().StringVar(&opts.sources, "source", "", "comma-separated source filter: claude,codex,gemini")
	cmd.PersistentFlags().StringVar(&opts.project, "project", "", "project/cwd substring filter")
	cmd.PersistentFlags().StringVar(&opts.model, "model", "", "model substring filter")
	cmd.PersistentFlags().StringVar(&opts.configPath, "config", "", "config file path")
	cmd.PersistentFlags().StringVar(&opts.refresh, "refresh", "", "live refresh interval, for example 2s")
	cmd.PersistentFlags().BoolVar(&opts.json, "json", false, "print JSON instead of a table")
}

func reportCommand(use, short string, opts *options, render func(*os.File, []usage.Event, report.Options), before func()) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if before != nil {
				before()
			}
			events, cfg, err := load(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if opts.json {
				return writeJSON(events, cfg)
			}
			render(os.Stdout, events, reportOptions(cfg))
			return nil
		},
	}
	return cmd
}

func load(ctx context.Context, opts *options) ([]usage.Event, config.Config, error) {
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return nil, config.Config{}, err
	}
	events, err := source.Collect(ctx, cfg)
	if err != nil {
		return nil, config.Config{}, err
	}
	since, until, err := window(opts)
	if err != nil {
		return nil, config.Config{}, err
	}
	var sources []string
	if opts.sources != "" {
		sources = strings.Split(opts.sources, ",")
	}
	return usage.Filter(events, since, until, sources, opts.project, opts.model), cfg, nil
}

func window(opts *options) (time.Time, time.Time, error) {
	var since, until time.Time
	var err error
	if opts.since != "" {
		since, err = time.Parse("2006-01-02", opts.since)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse --since: %w", err)
		}
	}
	if opts.until != "" {
		until, err = time.Parse("2006-01-02", opts.until)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse --until: %w", err)
		}
		until = until.Add(24*time.Hour - time.Nanosecond)
	}
	if since.IsZero() && opts.days > 0 {
		since = time.Now().AddDate(0, 0, -opts.days)
	}
	return since, until, nil
}

func writeJSON(events []usage.Event, cfg config.Config) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Totals usage.Totals  `json:"totals"`
		Events []usage.Event `json:"events"`
	}{Totals: usage.SummarizeWith(events, cfg.Pricing), Events: events})
}

func printPath(path string) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("  ok      %s\n", path)
	} else {
		fmt.Printf("  missing %s\n", path)
	}
}

func configCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configuration helpers"}
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print config path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(configPath(opts))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create a starter config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			if opts.configPath != "" {
				path = opts.configPath
			}
			if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
				return err
			}
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("config already exists: %s", path)
			}
			body := []byte(configTemplate())
			return os.WriteFile(path, body, 0o644)
		},
	})
	return cmd
}

func filepathDir(path string) string {
	return filepath.Dir(path)
}

func configPath(opts *options) string {
	if opts.configPath != "" {
		return opts.configPath
	}
	return config.ConfigPath()
}

func reportOptions(cfg config.Config) report.Options {
	return report.Options{Pricing: cfg.Pricing, Currency: cfg.Currency}
}

func tuiOptions(cfg config.Config, opts *options, reload func() ([]usage.Event, error), interval time.Duration) tui.Options {
	return tui.Options{
		Report:          reportOptions(cfg),
		RefreshInterval: interval,
		Reload:          reload,
		Filter:          filterLabel(opts),
		LogDirs:         append(append(append([]string{}, cfg.Paths.Claude...), cfg.Paths.Codex...), cfg.Paths.Gemini...),
	}
}

// filterLabel summarizes any active --since/--days/--source/--project/--model
// filters for display in the TUI sidebar (empty when nothing is filtered).
func filterLabel(opts *options) string {
	var parts []string
	if opts.sources != "" {
		parts = append(parts, "source="+opts.sources)
	}
	if opts.project != "" {
		parts = append(parts, "project="+opts.project)
	}
	if opts.model != "" {
		parts = append(parts, "model="+opts.model)
	}
	if opts.since != "" {
		parts = append(parts, "since="+opts.since)
	}
	if opts.until != "" {
		parts = append(parts, "until="+opts.until)
	}
	// days has a default (30), so it is part of the window, not an explicit
	// filter; the period labels on the panels already convey it.
	return strings.Join(parts, "\n")
}

func configTemplate() string {
	return `timezone = "local"
refresh_interval = "3s"
currency = "USD"

[paths]
claude = ["~/.claude/projects"]
codex = ["~/.codex/sessions", "~/.codex/archived_sessions"]
gemini = ["~/.gemini/tmp"]

# Prices are USD per million tokens. Keys match model substrings.
[pricing."claude-sonnet"]
input_per_million = 3
output_per_million = 15
cache_read_per_million = 0.30
cache_write_per_million = 3.75

[pricing."gpt-5"]
input_per_million = 1.25
output_per_million = 10
cache_read_per_million = 0.125
cache_write_per_million = 0

[pricing."codex"]
input_per_million = 1.25
output_per_million = 10
cache_read_per_million = 0.125
cache_write_per_million = 0

[pricing."gemini-2.5-pro"]
input_per_million = 1.25
output_per_million = 10
cache_read_per_million = 0
cache_write_per_million = 0
`
}
