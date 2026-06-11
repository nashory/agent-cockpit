package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nashory/agent-cockpit/internal/config"
	"github.com/nashory/agent-cockpit/internal/report"
	"github.com/nashory/agent-cockpit/internal/source"
	_ "github.com/nashory/agent-cockpit/internal/source/builtin"
	"github.com/nashory/agent-cockpit/internal/tui"
	"github.com/nashory/agent-cockpit/internal/usage"
	"github.com/nashory/agent-cockpit/internal/watch"
	"github.com/spf13/cobra"
)

type options struct {
	days         int
	since        string
	until        string
	sources      string
	project      string
	model        string
	configPath   string
	refresh      string
	timezone     string
	order        string
	breakdown    string
	json         bool
	noCost       bool
	svgPath      string
	compact      bool
	statusFormat string
	exportGroup  string
	outputPath   string
	statusline   *claudeStatuslineContext
}

var version = "dev"

const jsonSchemaVersion = "1"

type usageJSONDocument struct {
	SchemaVersion string                  `json:"schema_version"`
	GeneratedAt   time.Time               `json:"generated_at"`
	CostMode      string                  `json:"cost_mode"`
	Order         string                  `json:"order,omitempty"`
	Breakdown     string                  `json:"breakdown,omitempty"`
	Report        string                  `json:"report,omitempty"`
	Range         usageJSONRange          `json:"range"`
	Filters       usageJSONFilters        `json:"filters"`
	Totals        usageJSONTotals         `json:"totals"`
	Budgets       []usage.ThresholdStatus `json:"budgets,omitempty"`
	Limits        []usage.ThresholdStatus `json:"limits,omitempty"`
	Rows          any                     `json:"rows,omitempty"`
	Events        []usage.Event           `json:"events"`
}

type usageJSONRange struct {
	Days     int    `json:"days,omitempty"`
	Since    string `json:"since,omitempty"`
	Until    string `json:"until,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type usageJSONFilters struct {
	Sources []string `json:"sources,omitempty"`
	Project string   `json:"project,omitempty"`
	Model   string   `json:"model,omitempty"`
}

type usageJSONContext struct {
	Report    string
	NoCost    bool
	Loc       *time.Location
	Order     string
	Breakdown string
	Range     usageJSONRange
	Filters   usageJSONFilters
}

type usageJSONTotals struct {
	Events           int      `json:"events"`
	Input            int64    `json:"input_tokens"`
	Output           int64    `json:"output_tokens"`
	CacheRead        int64    `json:"cache_read_tokens"`
	CacheCreate      int64    `json:"cache_create_tokens"`
	Reasoning        int64    `json:"reasoning_tokens"`
	Total            int64    `json:"total_tokens"`
	EstimatedCostUSD *float64 `json:"estimated_cost_usd,omitempty"`
}

type bucketJSONRow struct {
	Key    string          `json:"key"`
	Totals usageJSONTotals `json:"totals"`
	Share  float64         `json:"share"`
}

type summaryJSONSection struct {
	Name string          `json:"name"`
	Rows []bucketJSONRow `json:"rows"`
}

type trendJSONRow struct {
	Date   string          `json:"date"`
	Totals usageJSONTotals `json:"totals"`
}

type speedJSONRow struct {
	Name            string  `json:"name"`
	Source          string  `json:"source"`
	Model           string  `json:"model,omitempty"`
	OutputTokens    int64   `json:"output_tokens"`
	Events          int     `json:"events"`
	WindowSeconds   int64   `json:"window_seconds"`
	OutputPerSecond float64 `json:"output_tokens_per_second"`
	FirstActivity   string  `json:"first_activity,omitempty"`
	LastActivity    string  `json:"last_activity,omitempty"`
}

type statuslineJSONDocument struct {
	SchemaVersion string                   `json:"schema_version"`
	GeneratedAt   time.Time                `json:"generated_at"`
	CostMode      string                   `json:"cost_mode"`
	Active        *statuslineActiveContext `json:"active,omitempty"`
	Totals        usageJSONTotals          `json:"totals"`
	Currency      string                   `json:"currency"`
	Budgets       []usage.ThresholdStatus  `json:"budgets,omitempty"`
	Limits        []usage.ThresholdStatus  `json:"limits,omitempty"`
}

type claudeStatuslineContext struct {
	CWD            string `json:"cwd"`
	SessionID      string `json:"session_id"`
	SessionName    string `json:"session_name"`
	TranscriptPath string `json:"transcript_path"`
	Version        string `json:"version"`
	Model          struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
	Cost struct {
		TotalCostUSD *float64 `json:"total_cost_usd"`
	} `json:"cost"`
	ContextWindow struct {
		ContextWindowSize int64    `json:"context_window_size"`
		UsedPercentage    *float64 `json:"used_percentage"`
		RemainingPercent  *float64 `json:"remaining_percentage"`
		TotalInputTokens  int64    `json:"total_input_tokens"`
		TotalOutputTokens int64    `json:"total_output_tokens"`
	} `json:"context_window"`
}

type statuslineActiveContext struct {
	Source                  string   `json:"source"`
	SessionID               string   `json:"session_id,omitempty"`
	SessionName             string   `json:"session_name,omitempty"`
	ModelID                 string   `json:"model_id,omitempty"`
	ModelName               string   `json:"model_name,omitempty"`
	CWD                     string   `json:"cwd,omitempty"`
	CurrentDir              string   `json:"current_dir,omitempty"`
	ProjectDir              string   `json:"project_dir,omitempty"`
	TranscriptPath          string   `json:"transcript_path,omitempty"`
	Version                 string   `json:"version,omitempty"`
	ContextWindowSize       int64    `json:"context_window_size,omitempty"`
	ContextUsedPercent      *float64 `json:"context_used_percentage,omitempty"`
	ContextRemainingPercent *float64 `json:"context_remaining_percentage,omitempty"`
	ContextInputTokens      int64    `json:"context_input_tokens,omitempty"`
	ContextOutputTokens     int64    `json:"context_output_tokens,omitempty"`
	SessionCostUSD          *float64 `json:"session_cost_usd,omitempty"`
}

type configValidationDocument struct {
	OK     bool                     `json:"ok"`
	Path   string                   `json:"path"`
	Errors []config.ValidationError `json:"errors"`
}

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
				return writeJSON(events, cfg, opts, "events")
			}
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			if _, _, err := locationFor(opts, cfg); err != nil {
				return err
			}
			if err := validateOrder(opts.order); err != nil {
				return err
			}
			if err := validateBreakdown(opts.breakdown); err != nil {
				return err
			}
			reload := func() ([]usage.Event, error) {
				events, _, err := load(cmd.Context(), opts)
				return events, err
			}
			tui.ApplyTerminalTheme()
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
		report.Buckets(w, "Agents", groupUsage(events, ro.Pricing, ro.NoCost, ro.Order, func(e usage.Event) string { return e.Source }), 0, ro)
	}, nil))
	root.AddCommand(reportCommand("sessions", "Show highest-usage sessions", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Sessions(w, events, 20, ro)
	}, nil))
	root.AddCommand(reportCommand("trends", "Show token and cost trends", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Trend(w, events, opts.days, ro)
	}, nil))
	root.AddCommand(reportCommand("speed", "Show observed output token speed by agent/model", opts, func(w *os.File, events []usage.Event, ro report.Options) {
		report.Speed(w, events, 20, ro)
	}, nil))
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Print a usage summary, or write a shareable SVG card with --svg",
		RunE: func(cmd *cobra.Command, args []string) error {
			events, cfg, err := load(cmd.Context(), opts)
			if err != nil {
				return err
			}
			ro := reportOptions(cfg, opts)
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
				return writeJSON(events, cfg, opts, "summary")
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
			if _, _, err := locationFor(opts, cfg); err != nil {
				return err
			}
			if err := validateOrder(opts.order); err != nil {
				return err
			}
			if err := validateBreakdown(opts.breakdown); err != nil {
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
			tui.ApplyTerminalTheme()
			_, err = tea.NewProgram(tui.New(nil, tuiOpts), tea.WithAltScreen()).Run()
			return err
		},
	})
	root.AddCommand(statuslineCommand(opts))
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export usage rows as CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			events, cfg, err := load(cmd.Context(), opts)
			if err != nil {
				return err
			}
			var w io.Writer = os.Stdout
			var f *os.File
			if opts.outputPath != "" {
				f, err = os.Create(opts.outputPath)
				if err != nil {
					return err
				}
				defer f.Close()
				w = f
			}
			return writeCSV(w, events, reportOptions(cfg, opts), opts.exportGroup)
		},
	}
	exportCmd.Flags().StringVar(&opts.exportGroup, "group", "daily", "CSV rows: daily, session, model, project, event")
	exportCmd.Flags().StringVarP(&opts.outputPath, "output", "o", "", "write CSV to a file instead of stdout")
	root.AddCommand(exportCmd)
	pricingCmd := &cobra.Command{Use: "pricing", Short: "Pricing diagnostics"}
	pricingCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show vendored pricing coverage for the current logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			events, cfg, err := load(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return writePricingStatus(os.Stdout, events, cfg, opts.json)
		},
	})
	root.AddCommand(pricingCmd)
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
	cmd.PersistentFlags().StringVar(&opts.since, "since", "", "start date or relative duration, for example YYYY-MM-DD, 7d, 2w, or 168h")
	cmd.PersistentFlags().StringVar(&opts.until, "until", "", "end date, YYYY-MM-DD")
	cmd.PersistentFlags().StringVar(&opts.sources, "source", "", "comma-separated source filter: claude,codex,gemini")
	cmd.PersistentFlags().StringVar(&opts.project, "project", "", "project/cwd substring filter")
	cmd.PersistentFlags().StringVar(&opts.model, "model", "", "model substring filter")
	cmd.PersistentFlags().StringVar(&opts.configPath, "config", "", "config file path")
	cmd.PersistentFlags().StringVar(&opts.refresh, "refresh", "", "live refresh interval, for example 2s")
	cmd.PersistentFlags().StringVar(&opts.timezone, "timezone", "", "IANA timezone for date windows, for example Europe/Zurich")
	cmd.PersistentFlags().StringVar(&opts.order, "order", "", "row order override: asc or desc")
	cmd.PersistentFlags().StringVar(&opts.breakdown, "breakdown", "", "aggregate breakdown override: source, model, or project")
	cmd.PersistentFlags().BoolVar(&opts.json, "json", false, "print JSON instead of a table")
	cmd.PersistentFlags().BoolVar(&opts.noCost, "no-cost", false, "omit estimated cost output where supported")
	cmd.PersistentFlags().BoolVar(&opts.compact, "compact", false, "print compact one-line output where supported")
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
				return writeJSON(events, cfg, opts, use)
			}
			render(os.Stdout, events, reportOptions(cfg, opts))
			return nil
		},
	}
	return cmd
}

func statuslineCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "statusline",
		Short: "Print one-line usage for tmux/statusline integrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := readClaudeStatuslineContext(os.Stdin)
			if err != nil {
				return err
			}
			opts.statusline = ctx
			events, cfg, err := load(cmd.Context(), opts)
			if err != nil {
				return err
			}
			writeStatusline(os.Stdout, events, reportOptions(cfg, opts), opts)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.statusFormat, "format", "", "custom statusline format, for example '{{model}} {{context}} {{today_cost}}'")
	return cmd
}

func load(ctx context.Context, opts *options) ([]usage.Event, config.Config, error) {
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return nil, config.Config{}, err
	}
	loc, _, err := locationFor(opts, cfg)
	if err != nil {
		return nil, config.Config{}, err
	}
	if err := validateOrder(opts.order); err != nil {
		return nil, config.Config{}, err
	}
	if err := validateBreakdown(opts.breakdown); err != nil {
		return nil, config.Config{}, err
	}
	events, err := source.Collect(ctx, cfg)
	if err != nil {
		return nil, config.Config{}, err
	}
	since, until, err := window(opts, loc, time.Now().In(loc))
	if err != nil {
		return nil, config.Config{}, err
	}
	var sources []string
	if opts.sources != "" {
		sources = strings.Split(opts.sources, ",")
	}
	return usage.Filter(events, since, until, sources, opts.project, opts.model), cfg, nil
}

func window(opts *options, loc *time.Location, now time.Time) (time.Time, time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	if now.IsZero() {
		now = time.Now().In(loc)
	}
	var since, until time.Time
	var err error
	if opts.since != "" {
		since, err = parseSince(opts.since, loc, now)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse --since: %w", err)
		}
	}
	if opts.until != "" {
		until, err = time.ParseInLocation("2006-01-02", opts.until, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse --until: %w", err)
		}
		until = until.Add(24*time.Hour - time.Nanosecond)
	}
	if since.IsZero() && opts.days > 0 {
		since = now.AddDate(0, 0, -opts.days)
	}
	return since, until, nil
}

func parseSince(raw string, loc *time.Location, now time.Time) (time.Time, error) {
	if t, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
		return t, nil
	}
	if d, ok, err := parseRelativeDateDuration(raw); ok || err != nil {
		if err != nil {
			return time.Time{}, err
		}
		return now.AddDate(0, 0, -d), nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return time.Time{}, err
	}
	if d <= 0 {
		return time.Time{}, fmt.Errorf("relative duration must be positive")
	}
	return now.Add(-d), nil
}

func parseRelativeDateDuration(raw string) (int, bool, error) {
	if len(raw) < 2 {
		return 0, false, nil
	}
	unit := raw[len(raw)-1]
	var multiplier int
	switch unit {
	case 'd':
		multiplier = 1
	case 'w':
		multiplier = 7
	default:
		return 0, false, nil
	}
	n, err := strconv.Atoi(raw[:len(raw)-1])
	if err != nil {
		return 0, true, err
	}
	if n <= 0 {
		return 0, true, fmt.Errorf("relative duration must be positive")
	}
	return n * multiplier, true, nil
}

func writeJSON(events []usage.Event, cfg config.Config, opts *options, reportName string) error {
	loc, timezone, err := locationFor(opts, cfg)
	if err != nil {
		return err
	}
	ctx := buildUsageJSONContext(opts, timezone)
	ctx.Loc = loc
	ctx.Report = reportName
	return writeUsageJSON(os.Stdout, events, cfg, time.Now().In(loc), ctx)
}

func buildUsageJSONContext(opts *options, timezone string) usageJSONContext {
	if opts == nil {
		return usageJSONContext{}
	}
	ctx := usageJSONContext{
		NoCost:    opts.noCost,
		Order:     opts.order,
		Breakdown: opts.breakdown,
		Range: usageJSONRange{
			Since:    opts.since,
			Until:    opts.until,
			Timezone: timezone,
		},
		Filters: usageJSONFilters{
			Project: opts.project,
			Model:   opts.model,
		},
	}
	if opts.since == "" && opts.days > 0 {
		ctx.Range.Days = opts.days
	}
	if opts.sources != "" {
		ctx.Filters.Sources = strings.Split(opts.sources, ",")
	}
	return ctx
}

func writeUsageJSON(w io.Writer, events []usage.Event, cfg config.Config, now time.Time, ctx usageJSONContext) error {
	totals := usage.SummarizeWith(events, cfg.Pricing)
	budgets := usage.BudgetStatuses(events, cfg.Pricing, cfg.Budget, now)
	if ctx.NoCost {
		totals = usage.SummarizeTokens(events)
		budgets = nil
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(usageJSONDocument{
		SchemaVersion: jsonSchemaVersion,
		GeneratedAt:   now,
		CostMode:      costMode(ctx.NoCost),
		Order:         ctx.Order,
		Breakdown:     ctx.Breakdown,
		Report:        ctx.Report,
		Range:         ctx.Range,
		Filters:       ctx.Filters,
		Totals:        usageTotalsJSON(totals, ctx.NoCost),
		Budgets:       budgets,
		Limits:        usage.ClaudeLimitStatuses(events, cfg.Pricing, cfg.Limits, now),
		Rows:          usageJSONRows(events, cfg, now, ctx),
		Events:        events,
	})
}

func costMode(noCost bool) string {
	if noCost {
		return "disabled"
	}
	return "estimated"
}

func usageTotalsJSON(t usage.Totals, noCost bool) usageJSONTotals {
	out := usageJSONTotals{
		Events:      t.Events,
		Input:       t.Input,
		Output:      t.Output,
		CacheRead:   t.CacheRead,
		CacheCreate: t.CacheCreate,
		Reasoning:   t.Reasoning,
		Total:       t.Total,
	}
	if !noCost {
		cost := t.CostUSD
		out.EstimatedCostUSD = &cost
	}
	return out
}

func usageJSONRows(events []usage.Event, cfg config.Config, now time.Time, ctx usageJSONContext) any {
	switch ctx.Report {
	case "today", "weekly", "monthly", "summary", "report":
		if ctx.Breakdown != "" {
			name, key := breakdownSpec(ctx.Breakdown)
			return []summaryJSONSection{
				{Name: name, Rows: bucketRows(events, cfg.Pricing, ctx.NoCost, ctx.Order, 8, key)},
			}
		}
		return []summaryJSONSection{
			{Name: "agents", Rows: bucketRows(events, cfg.Pricing, ctx.NoCost, ctx.Order, 8, func(e usage.Event) string { return e.Source })},
			{Name: "models", Rows: bucketRows(events, cfg.Pricing, ctx.NoCost, ctx.Order, 8, func(e usage.Event) string { return e.Model })},
		}
	case "agents":
		return bucketRows(events, cfg.Pricing, ctx.NoCost, ctx.Order, 0, func(e usage.Event) string { return e.Source })
	case "sessions":
		return sessionBuckets(events, cfg.Pricing, ctx.NoCost, ctx.Order, 20)
	case "trends":
		days := ctx.Range.Days
		if days <= 0 {
			days = 30
		}
		return trendRows(events, cfg.Pricing, ctx.NoCost, ctx.Order, days, now, ctx.Loc)
	case "speed":
		return speedRows(events, ctx.Order, 20)
	default:
		return nil
	}
}

func bucketRows(events []usage.Event, prices usage.PriceBook, noCost bool, order string, limit int, key func(usage.Event) string) []bucketJSONRow {
	buckets := groupUsage(events, prices, noCost, order, key)
	buckets = limitBuckets(buckets, limit)
	rows := make([]bucketJSONRow, 0, len(buckets))
	for _, b := range buckets {
		rows = append(rows, bucketJSONRow{
			Key:    b.Key,
			Totals: usageTotalsJSON(b.Totals, noCost),
			Share:  b.Share,
		})
	}
	return rows
}

func limitBuckets(buckets []usage.Bucket, limit int) []usage.Bucket {
	if limit <= 0 || len(buckets) <= limit {
		return buckets
	}
	return buckets[:limit]
}

func sessionBuckets(events []usage.Event, prices usage.PriceBook, noCost bool, order string, limit int) []bucketJSONRow {
	return bucketRows(events, prices, noCost, order, limit, func(e usage.Event) string {
		if e.Project != "" && e.SessionID != "" {
			return e.Project + " / " + shortID(e.SessionID)
		}
		return e.SessionID
	})
}

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

func trendRows(events []usage.Event, prices usage.PriceBook, noCost bool, order string, days int, now time.Time, loc *time.Location) []trendJSONRow {
	if loc == nil {
		loc = time.Local
	}
	if now.IsZero() {
		now = time.Now().In(loc)
	}
	now = now.In(loc)
	if days <= 0 {
		days = 30
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -days+1)
	byDay := make([][]usage.Event, days)
	for _, e := range events {
		ts := e.Timestamp.In(loc)
		day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, loc)
		idx := int(day.Sub(start) / (24 * time.Hour))
		if idx >= 0 && idx < days {
			byDay[idx] = append(byDay[idx], e)
		}
	}
	rows := make([]trendJSONRow, 0, days)
	for i := range byDay {
		totals := usage.SummarizeWith(byDay[i], prices)
		if noCost {
			totals = usage.SummarizeTokens(byDay[i])
		}
		rows = append(rows, trendJSONRow{
			Date:   start.AddDate(0, 0, i).Format("2006-01-02"),
			Totals: usageTotalsJSON(totals, noCost),
		})
	}
	if order == "desc" {
		reverseTrendRows(rows)
	}
	return rows
}

func speedRows(events []usage.Event, order string, limit int) []speedJSONRow {
	type stats struct {
		source string
		model  string
		first  time.Time
		last   time.Time
		tokens int64
		events int
	}
	byKey := map[string]*stats{}
	tokensPerSecond := func(s *stats) float64 {
		seconds := s.last.Sub(s.first).Seconds()
		if seconds <= 0 {
			return 0
		}
		return float64(s.tokens) / seconds
	}
	for _, e := range events {
		key := e.Source + "\x00" + e.Model
		s := byKey[key]
		if s == nil {
			s = &stats{source: e.Source, model: e.Model, first: e.Timestamp, last: e.Timestamp}
			byKey[key] = s
		}
		if !e.Timestamp.IsZero() {
			if s.first.IsZero() || e.Timestamp.Before(s.first) {
				s.first = e.Timestamp
			}
			if e.Timestamp.After(s.last) {
				s.last = e.Timestamp
			}
		}
		s.tokens += e.Output
		s.events++
	}
	statsRows := make([]*stats, 0, len(byKey))
	for _, s := range byKey {
		statsRows = append(statsRows, s)
	}
	sort.Slice(statsRows, func(i, j int) bool {
		return tokensPerSecond(statsRows[i]) > tokensPerSecond(statsRows[j])
	})
	if order == "asc" {
		reverseStatsRows(statsRows)
	}
	if limit > 0 && len(statsRows) > limit {
		statsRows = statsRows[:limit]
	}
	rows := make([]speedJSONRow, 0, len(statsRows))
	for _, s := range statsRows {
		name := s.source
		if s.model != "" {
			name += " / " + s.model
		}
		row := speedJSONRow{
			Name:            name,
			Source:          s.source,
			Model:           s.model,
			OutputTokens:    s.tokens,
			Events:          s.events,
			WindowSeconds:   int64(s.last.Sub(s.first).Seconds()),
			OutputPerSecond: tokensPerSecond(s),
		}
		if !s.first.IsZero() {
			row.FirstActivity = s.first.Format(time.RFC3339)
		}
		if !s.last.IsZero() {
			row.LastActivity = s.last.Format(time.RFC3339)
		}
		rows = append(rows, row)
	}
	return rows
}

func readClaudeStatuslineContext(stdin *os.File) (*claudeStatuslineContext, error) {
	if stdin == nil {
		return nil, nil
	}
	st, err := stdin.Stat()
	if err != nil {
		return nil, err
	}
	if st.Mode()&os.ModeCharDevice != 0 {
		return nil, nil
	}
	b, err := io.ReadAll(stdin)
	if err != nil {
		return nil, err
	}
	return parseClaudeStatuslineInput(b)
}

func parseClaudeStatuslineInput(b []byte) (*claudeStatuslineContext, error) {
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil, nil
	}
	var ctx claudeStatuslineContext
	if err := json.Unmarshal(b, &ctx); err != nil {
		return nil, fmt.Errorf("parse Claude statusline stdin: %w", err)
	}
	if ctx.SessionID == "" && ctx.Model.ID == "" && ctx.Model.DisplayName == "" && ctx.CWD == "" && ctx.Workspace.CurrentDir == "" {
		return nil, nil
	}
	return &ctx, nil
}

func writeStatusline(w io.Writer, events []usage.Event, ro report.Options, opts *options) {
	active := statuslineActive(opts.statusline)
	events = statuslineEvents(events, opts.statusline)
	t := usage.SummarizeWith(events, ro.Pricing)
	if ro.NoCost {
		t = usage.SummarizeTokens(events)
	}
	currency := ro.Currency
	if currency == "" {
		currency = "USD"
	}
	cfg, _ := config.Load(opts.configPath)
	now := time.Now()
	if ro.Location != nil {
		now = now.In(ro.Location)
	}
	var budgets []usage.ThresholdStatus
	if !ro.NoCost {
		budgets = usage.BudgetStatuses(events, ro.Pricing, cfg.Budget, now)
	}
	limits := usage.ClaudeLimitStatuses(events, ro.Pricing, cfg.Limits, now)
	if opts.json {
		_ = writeStatuslineJSON(w, t, currency, budgets, limits, now, ro.NoCost, active)
		return
	}
	if opts.statusFormat != "" {
		values := statuslineFormatValues(t, events, ro, active, budgets, limits, now, currency)
		fmt.Fprintln(w, renderStatuslineFormat(opts.statusFormat, values))
		return
	}
	if opts.compact {
		parts := statuslineActiveParts(active)
		parts = append(parts, fmt.Sprintf("tok %s", formatCompact(t.Total)))
		if !ro.NoCost {
			parts = append(parts, fmt.Sprintf("~%.2f %s", t.CostUSD, currency))
		}
		if s := usage.WorstStatus(append(budgets, limits...)); s.Name != "" {
			parts = append(parts, fmt.Sprintf("%s %.0f%% %s", s.Name, s.Ratio*100, s.Level))
		}
		fmt.Fprintln(w, strings.Join(parts, " | "))
		return
	}
	if parts := statuslineActiveParts(active); len(parts) > 0 {
		fmt.Fprintf(w, "%s | ", strings.Join(parts, " | "))
	}
	fmt.Fprintf(w, "tokens %d", t.Total)
	if !ro.NoCost {
		fmt.Fprintf(w, " | cost %.2f %s", t.CostUSD, currency)
	}
	fmt.Fprintf(w, " | events %d", t.Events)
	if s := usage.WorstStatus(append(budgets, limits...)); s.Name != "" {
		fmt.Fprintf(w, " | %s %.0f%% %s", s.Name, s.Ratio*100, s.Level)
	}
	fmt.Fprintln(w)
}

func statuslineEvents(events []usage.Event, ctx *claudeStatuslineContext) []usage.Event {
	if ctx == nil || ctx.SessionID == "" {
		return events
	}
	var filtered []usage.Event
	for _, e := range events {
		if e.Source == "claude" && e.SessionID == ctx.SessionID {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		return events
	}
	return filtered
}

func statuslineActive(ctx *claudeStatuslineContext) *statuslineActiveContext {
	if ctx == nil {
		return nil
	}
	return &statuslineActiveContext{
		Source:                  "claude",
		SessionID:               ctx.SessionID,
		SessionName:             ctx.SessionName,
		ModelID:                 ctx.Model.ID,
		ModelName:               statuslineModelName(ctx),
		CWD:                     ctx.CWD,
		CurrentDir:              ctx.Workspace.CurrentDir,
		ProjectDir:              ctx.Workspace.ProjectDir,
		TranscriptPath:          ctx.TranscriptPath,
		Version:                 ctx.Version,
		ContextWindowSize:       ctx.ContextWindow.ContextWindowSize,
		ContextUsedPercent:      ctx.ContextWindow.UsedPercentage,
		ContextRemainingPercent: ctx.ContextWindow.RemainingPercent,
		ContextInputTokens:      ctx.ContextWindow.TotalInputTokens,
		ContextOutputTokens:     ctx.ContextWindow.TotalOutputTokens,
		SessionCostUSD:          ctx.Cost.TotalCostUSD,
	}
}

func statuslineActiveParts(active *statuslineActiveContext) []string {
	if active == nil {
		return nil
	}
	var parts []string
	if active.ModelName != "" {
		parts = append(parts, "model "+active.ModelName)
	}
	if active.ContextUsedPercent != nil {
		parts = append(parts, fmt.Sprintf("ctx %.0f%%", *active.ContextUsedPercent))
	}
	return parts
}

func statuslineFormatValues(totals usage.Totals, events []usage.Event, ro report.Options, active *statuslineActiveContext, budgets, limits []usage.ThresholdStatus, now time.Time, currency string) map[string]string {
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today := usage.SummarizeWith(usage.Filter(events, todayStart, time.Time{}, nil, "", ""), ro.Pricing)
	if ro.NoCost {
		today = usage.SummarizeTokens(usage.Filter(events, todayStart, time.Time{}, nil, "", ""))
	}
	values := map[string]string{
		"tokens":         fmt.Sprintf("%d", totals.Total),
		"tokens_compact": formatCompact(totals.Total),
		"events":         fmt.Sprintf("%d", totals.Events),
		"today_tokens":   fmt.Sprintf("%d", today.Total),
		"today_compact":  formatCompact(today.Total),
		"cost":           "",
		"today_cost":     "",
		"budget":         statuslineWorstLabel(budgets),
		"limit":          statuslineWorstLabel(limits),
		"block_left":     statuslineBlockLeft(limits),
		"model":          "",
		"model_id":       "",
		"session":        "",
		"context":        "",
		"context_left":   "",
		"cwd":            "",
		"project":        "",
	}
	if !ro.NoCost {
		values["cost"] = fmt.Sprintf("~%.2f %s", totals.CostUSD, currency)
		values["today_cost"] = fmt.Sprintf("~%.2f %s", today.CostUSD, currency)
	}
	if active != nil {
		values["model"] = active.ModelName
		values["model_id"] = active.ModelID
		values["session"] = active.SessionID
		values["cwd"] = active.CWD
		values["project"] = active.ProjectDir
		if active.ProjectDir == "" {
			values["project"] = active.CurrentDir
		}
		if active.ContextUsedPercent != nil {
			values["context"] = fmt.Sprintf("%.0f%%", *active.ContextUsedPercent)
		}
		if active.ContextRemainingPercent != nil {
			values["context_left"] = fmt.Sprintf("%.0f%%", *active.ContextRemainingPercent)
		}
	}
	return values
}

func renderStatuslineFormat(format string, values map[string]string) string {
	out := format
	for key, value := range values {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
		out = strings.ReplaceAll(out, "{{ "+key+" }}", value)
	}
	return out
}

func statuslineWorstLabel(statuses []usage.ThresholdStatus) string {
	s := usage.WorstStatus(statuses)
	if s.Name == "" {
		return ""
	}
	return fmt.Sprintf("%s %.0f%% %s", s.Name, s.Ratio*100, s.Level)
}

func statuslineBlockLeft(statuses []usage.ThresholdStatus) string {
	for _, s := range statuses {
		if s.Name != "claude 5h" || s.Limit <= 0 {
			continue
		}
		left := s.Limit - s.Used
		if left < 0 {
			left = 0
		}
		return formatCompact(int64(left))
	}
	return ""
}

func statuslineModelName(ctx *claudeStatuslineContext) string {
	if ctx.Model.DisplayName != "" {
		return ctx.Model.DisplayName
	}
	return ctx.Model.ID
}

func writeStatuslineJSON(w io.Writer, totals usage.Totals, currency string, budgets, limits []usage.ThresholdStatus, now time.Time, noCost bool, active *statuslineActiveContext) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(statuslineJSONDocument{
		SchemaVersion: jsonSchemaVersion,
		GeneratedAt:   now,
		CostMode:      costMode(noCost),
		Active:        active,
		Totals:        usageTotalsJSON(totals, noCost),
		Currency:      currency,
		Budgets:       budgets,
		Limits:        limits,
	})
}

func writeCSV(w io.Writer, events []usage.Event, ro report.Options, group string) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	cur := ro.Currency
	if cur == "" {
		cur = "USD"
	}
	writeTotals := func(key string, t usage.Totals) []string {
		if ro.NoCost {
			return []string{key, fmt.Sprint(t.Events), fmt.Sprint(t.Input), fmt.Sprint(t.Output), fmt.Sprint(t.CacheRead + t.CacheCreate), fmt.Sprint(t.Reasoning), fmt.Sprint(t.Total)}
		}
		return []string{key, fmt.Sprint(t.Events), fmt.Sprint(t.Input), fmt.Sprint(t.Output), fmt.Sprint(t.CacheRead + t.CacheCreate), fmt.Sprint(t.Reasoning), fmt.Sprint(t.Total), fmt.Sprintf("%.6f", t.CostUSD), cur}
	}
	header := func(key string) []string {
		fields := []string{key, "events", "input_tokens", "output_tokens", "cache_tokens", "reasoning_tokens", "total_tokens"}
		if !ro.NoCost {
			fields = append(fields, "estimated_cost", "currency")
		}
		return fields
	}
	if ro.Breakdown != "" && (group == "" || group == "daily") {
		group = ro.Breakdown
	}
	switch group {
	case "", "daily":
		if err := cw.Write(header("date")); err != nil {
			return err
		}
		for _, b := range timeBuckets(events, ro.Pricing, ro.NoCost, ro.Order, ro.Location, "day") {
			if err := cw.Write(writeTotals(b.Key, b.Totals)); err != nil {
				return err
			}
		}
	case "session":
		if err := cw.Write(header("session")); err != nil {
			return err
		}
		for _, b := range groupUsage(events, ro.Pricing, ro.NoCost, ro.Order, func(e usage.Event) string { return e.SessionID }) {
			if err := cw.Write(writeTotals(b.Key, b.Totals)); err != nil {
				return err
			}
		}
	case "model":
		if err := cw.Write(header("model")); err != nil {
			return err
		}
		for _, b := range groupUsage(events, ro.Pricing, ro.NoCost, ro.Order, func(e usage.Event) string { return e.Model }) {
			if err := cw.Write(writeTotals(b.Key, b.Totals)); err != nil {
				return err
			}
		}
	case "source":
		if err := cw.Write(header("source")); err != nil {
			return err
		}
		for _, b := range groupUsage(events, ro.Pricing, ro.NoCost, ro.Order, func(e usage.Event) string { return e.Source }) {
			if err := cw.Write(writeTotals(b.Key, b.Totals)); err != nil {
				return err
			}
		}
	case "project":
		if err := cw.Write(header("project")); err != nil {
			return err
		}
		for _, b := range groupUsage(events, ro.Pricing, ro.NoCost, ro.Order, func(e usage.Event) string { return e.Project }) {
			if err := cw.Write(writeTotals(b.Key, b.Totals)); err != nil {
				return err
			}
		}
	case "event":
		eventHeader := []string{"timestamp", "source", "project", "session", "model", "input_tokens", "output_tokens", "cache_tokens", "reasoning_tokens", "total_tokens"}
		if !ro.NoCost {
			eventHeader = append(eventHeader, "estimated_cost", "currency")
		}
		if err := cw.Write(eventHeader); err != nil {
			return err
		}
		eventRows := append([]usage.Event(nil), events...)
		sortEventRows(eventRows, ro.Order)
		for _, e := range eventRows {
			row := []string{
				e.Timestamp.Format(time.RFC3339), e.Source, e.Project, e.SessionID, e.Model,
				fmt.Sprint(e.Input), fmt.Sprint(e.Output), fmt.Sprint(e.CacheRead + e.CacheCreate), fmt.Sprint(e.Reasoning),
				fmt.Sprint(e.TotalTokens()),
			}
			if !ro.NoCost {
				row = append(row, fmt.Sprintf("%.6f", usage.EstimateCostWith(e, ro.Pricing)), cur)
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown export group %q", group)
	}
	return cw.Error()
}

func timeBuckets(events []usage.Event, prices usage.PriceBook, noCost bool, order string, loc *time.Location, period string) []usage.Bucket {
	if loc == nil {
		loc = time.Local
	}
	key := func(e usage.Event) string {
		if e.Timestamp.IsZero() {
			return "unknown"
		}
		ts := e.Timestamp.In(loc)
		switch period {
		case "week":
			y, w := ts.ISOWeek()
			return fmt.Sprintf("%04d-W%02d", y, w)
		case "month":
			return ts.Format("2006-01")
		default:
			return ts.Format("2006-01-02")
		}
	}
	rows := groupUsage(events, prices, noCost, "", key)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key > rows[j].Key })
	if order == "asc" {
		reverseBuckets(rows)
	}
	return rows
}

func groupUsage(events []usage.Event, prices usage.PriceBook, noCost bool, order string, key func(usage.Event) string) []usage.Bucket {
	var rows []usage.Bucket
	if noCost {
		rows = usage.GroupByTokens(events, key)
	} else {
		rows = usage.GroupByWith(events, prices, key)
	}
	if order == "asc" {
		reverseBuckets(rows)
	}
	return rows
}

func breakdownSpec(name string) (string, func(usage.Event) string) {
	switch name {
	case "project":
		return "projects", func(e usage.Event) string { return e.Project }
	case "model":
		return "models", func(e usage.Event) string { return e.Model }
	default:
		return "sources", func(e usage.Event) string { return e.Source }
	}
}

func reverseBuckets(rows []usage.Bucket) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}

func reverseTrendRows(rows []trendJSONRow) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}

func reverseStatsRows[T any](rows []*T) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}

func sortEventRows(rows []usage.Event, order string) {
	if order == "" {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if order == "asc" {
			return rows[i].Timestamp.Before(rows[j].Timestamp)
		}
		return rows[i].Timestamp.After(rows[j].Timestamp)
	})
}

func validateOrder(order string) error {
	switch order {
	case "", "asc", "desc":
		return nil
	default:
		return fmt.Errorf("invalid --order %q: expected asc or desc", order)
	}
}

func validateBreakdown(breakdown string) error {
	switch breakdown {
	case "", "source", "model", "project":
		return nil
	default:
		return fmt.Errorf("invalid --breakdown %q: expected source, model, or project", breakdown)
	}
}

func writePricingStatus(w io.Writer, events []usage.Event, cfg config.Config, asJSON bool) error {
	type row struct {
		Model  string `json:"model"`
		Source string `json:"pricing_source"`
		Events int    `json:"events"`
		Tokens int64  `json:"tokens"`
	}
	byModel := map[string]*row{}
	for _, e := range events {
		model := e.Model
		if model == "" {
			model = "unknown"
		}
		r := byModel[model]
		if r == nil {
			_, src := usage.ResolvePricing(model, cfg.Pricing)
			r = &row{Model: model, Source: src}
			byModel[model] = r
		}
		r.Events++
		r.Tokens += e.TotalTokens()
	}
	rows := make([]row, 0, len(byModel))
	for _, r := range byModel {
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Tokens > rows[j].Tokens })
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			VendoredModels  int   `json:"vendored_models"`
			ConfigOverrides int   `json:"config_overrides"`
			Models          []row `json:"models"`
		}{VendoredModels: usage.VendoredPricingCount(), ConfigOverrides: len(cfg.Pricing), Models: rows})
	}
	fmt.Fprintf(w, "Vendored pricing models: %d\n", usage.VendoredPricingCount())
	fmt.Fprintf(w, "Config overrides: %d\n\n", len(cfg.Pricing))
	fmt.Fprintln(w, "Model pricing coverage")
	for _, r := range rows {
		fmt.Fprintf(w, "  %-28s %-18s %8s tokens  %d events\n", truncateASCII(r.Model, 28), r.Source, formatCompact(r.Tokens), r.Events)
	}
	return nil
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
	cmd.AddCommand(&cobra.Command{
		Use:   "schema",
		Short: "Print the JSON schema for config.toml",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := config.SchemaJSON()
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, string(body))
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate config.toml",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := configPath(opts)
			errs := config.ValidateFile(path)
			if opts.json {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(configValidationDocument{OK: len(errs) == 0, Path: path, Errors: errs})
			}
			if len(errs) == 0 {
				fmt.Fprintf(os.Stdout, "config ok: %s\n", path)
				return nil
			}
			fmt.Fprintf(os.Stderr, "config invalid: %s\n", path)
			for _, err := range errs {
				fmt.Fprintf(os.Stderr, "  %s: %s\n", err.Field, err.Message)
			}
			return fmt.Errorf("config validation failed")
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

func reportOptions(cfg config.Config, opts *options) report.Options {
	noCost := opts != nil && opts.noCost
	var order, breakdown string
	if opts != nil {
		order = opts.order
		breakdown = opts.breakdown
	}
	loc, _, _ := locationFor(opts, cfg)
	return report.Options{Pricing: cfg.Pricing, Currency: cfg.Currency, Budget: cfg.Budget, Limits: cfg.Limits, NoCost: noCost, Location: loc, Order: order, Breakdown: breakdown}
}

func locationFor(opts *options, cfg config.Config) (*time.Location, string, error) {
	name := cfg.Timezone
	if opts != nil && opts.timezone != "" {
		name = opts.timezone
	}
	if name == "" || name == "local" {
		return time.Local, "", nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, "", fmt.Errorf("load timezone %q: %w", name, err)
	}
	return loc, name, nil
}

func tuiOptions(cfg config.Config, opts *options, reload func() ([]usage.Event, error), interval time.Duration) tui.Options {
	return tui.Options{
		Report:          reportOptions(cfg, opts),
		RefreshInterval: interval,
		Reload:          reload,
		Filter:          filterLabel(opts),
		LogDirs:         append(append(append([]string{}, cfg.Paths.Claude...), cfg.Paths.Codex...), cfg.Paths.Gemini...),
		RestorePrefs:    true,
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

func formatCompact(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprint(n)
	}
}

func truncateASCII(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "~"
}

func configTemplate() string {
	return `timezone = "local"
refresh_interval = "3s"
currency = "USD"

[budget]
# daily_usd = 25
# weekly_usd = 100
# monthly_usd = 300
# warn_pct = 80
# critical_pct = 95

[limits]
# claude_5h_tokens = 88000
# claude_7d_tokens = 500000
# warn_pct = 80
# critical_pct = 95

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
