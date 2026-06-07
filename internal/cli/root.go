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
	"github.com/spf13/cobra"
)

type options struct {
	days    int
	since   string
	until   string
	sources string
	project string
	model   string
	json    bool
}

var version = "dev"

func Execute() error {
	opts := &options{days: 30}
	root := &cobra.Command{
		Use:   "ac",
		Short: "Live usage, cost, and speed dashboards for coding agents",
		Long:  "ac is the command-line cockpit for local coding-agent logs: token usage, estimated cost, and trends without uploading your data.",
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := load(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if opts.json {
				return writeJSON(events)
			}
			_, err = tea.NewProgram(tui.New(events), tea.WithAltScreen()).Run()
			return err
		},
	}
	addFlags(root, opts)
	root.Version = version

	root.AddCommand(reportCommand("today", "Show today's static report", opts, func(w *os.File, events []usage.Event) {
		report.Overview(w, "Today", events)
	}, func() { opts.days = 1 }))
	root.AddCommand(reportCommand("weekly", "Show the last 7 days", opts, func(w *os.File, events []usage.Event) {
		report.Overview(w, "Last 7 days", events)
	}, func() { opts.days = 7 }))
	root.AddCommand(reportCommand("monthly", "Show the last 30 days", opts, func(w *os.File, events []usage.Event) {
		report.Overview(w, "Last 30 days", events)
	}, func() { opts.days = 30 }))
	root.AddCommand(reportCommand("agents", "Group usage by agent", opts, func(w *os.File, events []usage.Event) {
		report.Buckets(w, "Agents", usage.GroupBy(events, func(e usage.Event) string { return e.Source }), 0)
	}, nil))
	root.AddCommand(reportCommand("sessions", "Show highest-usage sessions", opts, func(w *os.File, events []usage.Event) {
		report.Sessions(w, events, 20)
	}, nil))
	root.AddCommand(reportCommand("trends", "Show token and cost trends", opts, func(w *os.File, events []usage.Event) {
		report.Trend(w, events, opts.days)
	}, nil))
	root.AddCommand(reportCommand("statusline", "Print one-line usage for tmux/statusline integrations", opts, func(w *os.File, events []usage.Event) {
		t := usage.Summarize(events)
		fmt.Fprintf(w, "tokens %d | cost $%.2f | events %d\n", t.Total, t.CostUSD, t.Events)
	}, nil))
	root.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Show detected log locations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Default()
			fmt.Println("Claude paths:")
			for _, p := range cfg.ClaudePaths {
				printPath(p)
			}
			fmt.Println("Codex paths:")
			for _, p := range cfg.CodexPaths {
				printPath(p)
			}
			return nil
		},
	})
	root.AddCommand(configCommand())

	return root.Execute()
}

func addFlags(cmd *cobra.Command, opts *options) {
	cmd.PersistentFlags().IntVar(&opts.days, "days", opts.days, "number of days to include")
	cmd.PersistentFlags().StringVar(&opts.since, "since", "", "start date, YYYY-MM-DD")
	cmd.PersistentFlags().StringVar(&opts.until, "until", "", "end date, YYYY-MM-DD")
	cmd.PersistentFlags().StringVar(&opts.sources, "source", "", "comma-separated source filter: claude,codex")
	cmd.PersistentFlags().StringVar(&opts.project, "project", "", "project/cwd substring filter")
	cmd.PersistentFlags().StringVar(&opts.model, "model", "", "model substring filter")
	cmd.PersistentFlags().BoolVar(&opts.json, "json", false, "print JSON instead of a table")
}

func reportCommand(use, short string, opts *options, render func(*os.File, []usage.Event), before func()) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if before != nil {
				before()
			}
			events, err := load(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if opts.json {
				return writeJSON(events)
			}
			render(os.Stdout, events)
			return nil
		},
	}
	return cmd
}

func load(ctx context.Context, opts *options) ([]usage.Event, error) {
	events, err := source.Collect(ctx, config.Default())
	if err != nil {
		return nil, err
	}
	since, until, err := window(opts)
	if err != nil {
		return nil, err
	}
	var sources []string
	if opts.sources != "" {
		sources = strings.Split(opts.sources, ",")
	}
	return usage.Filter(events, since, until, sources, opts.project, opts.model), nil
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

func writeJSON(events []usage.Event) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Totals usage.Totals  `json:"totals"`
		Events []usage.Event `json:"events"`
	}{Totals: usage.Summarize(events), Events: events})
}

func printPath(path string) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("  ok      %s\n", path)
	} else {
		fmt.Printf("  missing %s\n", path)
	}
}

func configCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configuration helpers"}
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print config path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.ConfigPath())
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create a starter config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
				return err
			}
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("config already exists: %s", path)
			}
			body := []byte("timezone = \"local\"\nrefresh_interval = \"3s\"\ncurrency = \"USD\"\n")
			return os.WriteFile(path, body, 0o644)
		},
	})
	return cmd
}

func filepathDir(path string) string {
	return filepath.Dir(path)
}
