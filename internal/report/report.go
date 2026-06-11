package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nashory/agent-cockpit/internal/usage"
)

type Options struct {
	Pricing  usage.PriceBook
	Currency string
	Budget   usage.Budget
	Limits   usage.Limits
	NoCost   bool
}

type speedStats struct {
	key    string
	first  time.Time
	last   time.Time
	tokens int64
	events int
}

func (o Options) currency() string {
	if o.Currency == "" {
		return "USD"
	}
	return o.Currency
}

func Overview(w io.Writer, title string, events []usage.Event, opts Options) {
	t := summarize(events, opts)
	fmt.Fprintf(w, "%s\n\n", title)
	fmt.Fprintf(w, "Events: %d\n", t.Events)
	fmt.Fprintf(w, "Tokens: %s total  %s input  %s output  %s cache  %s reasoning\n",
		formatInt(t.Total), formatInt(t.Input), formatInt(t.Output), formatInt(t.CacheRead+t.CacheCreate), formatInt(t.Reasoning))
	if !opts.NoCost {
		fmt.Fprintf(w, "Estimated cost: %.2f %s\n", t.CostUSD, opts.currency())
	}
	fmt.Fprintln(w)
	Buckets(w, "Agents", groupBy(events, opts, func(e usage.Event) string { return e.Source }), 8, opts)
	fmt.Fprintln(w)
	Buckets(w, "Models", groupBy(events, opts, func(e usage.Event) string { return e.Model }), 8, opts)
}

func Buckets(w io.Writer, title string, buckets []usage.Bucket, limit int, opts Options) {
	fmt.Fprintln(w, title)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if opts.NoCost {
		fmt.Fprintln(tw, "Name\tShare\tTokens")
	} else {
		fmt.Fprintln(tw, "Name\tShare\tTokens\tCost")
	}
	for i, b := range buckets {
		if limit > 0 && i >= limit {
			break
		}
		if opts.NoCost {
			fmt.Fprintf(tw, "%s\t%.1f%%\t%s\n", b.Key, b.Share*100, formatInt(b.Totals.Total))
		} else {
			fmt.Fprintf(tw, "%s\t%.1f%%\t%s\t%.2f %s\n", b.Key, b.Share*100, formatInt(b.Totals.Total), b.Totals.CostUSD, opts.currency())
		}
	}
	_ = tw.Flush()
}

func Sessions(w io.Writer, events []usage.Event, limit int, opts Options) {
	buckets := groupBy(events, opts, func(e usage.Event) string {
		if e.Project != "" && e.SessionID != "" {
			return e.Project + " / " + short(e.SessionID)
		}
		return e.SessionID
	})
	Buckets(w, "Sessions", buckets, limit, opts)
}

func Speed(w io.Writer, events []usage.Event, limit int) {
	byKey := map[string]*speedStats{}
	for _, e := range events {
		key := e.Source
		if e.Model != "" {
			key += " / " + e.Model
		}
		s, ok := byKey[key]
		if !ok {
			s = &speedStats{key: key, first: e.Timestamp, last: e.Timestamp}
			byKey[key] = s
		}
		if e.Timestamp.Before(s.first) {
			s.first = e.Timestamp
		}
		if e.Timestamp.After(s.last) {
			s.last = e.Timestamp
		}
		s.tokens += e.Output
		s.events++
	}

	rows := make([]*speedStats, 0, len(byKey))
	for _, s := range byKey {
		rows = append(rows, s)
	}
	sort.Slice(rows, func(i, j int) bool {
		return tokensPerSecond(rows[i]) > tokensPerSecond(rows[j])
	})

	fmt.Fprintln(w, "Observed Speed")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Agent / model\tOutput t/s\tOutput tokens\tWindow\tEvents")
	for i, row := range rows {
		if limit > 0 && i >= limit {
			break
		}
		fmt.Fprintf(tw, "%s\t%.2f\t%s\t%s\t%d\n", row.key, tokensPerSecond(row), formatInt(row.tokens), duration(row), row.events)
	}
	_ = tw.Flush()
}

func Trend(w io.Writer, events []usage.Event, days int, opts Options) {
	if days <= 0 {
		days = 30
	}
	start := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)
	byDay := make([][]usage.Event, days)
	for _, e := range events {
		idx := int(e.Timestamp.Truncate(24*time.Hour).Sub(start) / (24 * time.Hour))
		if idx >= 0 && idx < days {
			byDay[idx] = append(byDay[idx], e)
		}
	}
	max := int64(1)
	totals := make([]usage.Totals, days)
	for i := range byDay {
		totals[i] = summarize(byDay[i], opts)
		if totals[i].Total > max {
			max = totals[i].Total
		}
	}
	for i, t := range totals {
		day := start.AddDate(0, 0, i).Format("Jan 02")
		if opts.NoCost {
			fmt.Fprintf(w, "%s  %-24s %s\n", day, bar(t.Total, max, 24), formatInt(t.Total))
		} else {
			fmt.Fprintf(w, "%s  %-24s %s  %.2f %s\n", day, bar(t.Total, max, 24), formatInt(t.Total), t.CostUSD, opts.currency())
		}
	}
}

func summarize(events []usage.Event, opts Options) usage.Totals {
	if opts.NoCost {
		return usage.SummarizeTokens(events)
	}
	return usage.SummarizeWith(events, opts.Pricing)
}

func groupBy(events []usage.Event, opts Options, key func(usage.Event) string) []usage.Bucket {
	if opts.NoCost {
		return usage.GroupByTokens(events, key)
	}
	buckets := usage.GroupByWith(events, opts.Pricing, key)
	return buckets
}

func tokensPerSecond(row *speedStats) float64 {
	seconds := row.last.Sub(row.first).Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(row.tokens) / seconds
}

func duration(row *speedStats) string {
	d := row.last.Sub(row.first)
	if d <= 0 {
		return "n/a"
	}
	return d.Round(time.Second).String()
}

func formatInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}

func bar(v, max int64, width int) string {
	if v <= 0 {
		return strings.Repeat(".", width)
	}
	n := int(float64(v) / float64(max) * float64(width))
	if n < 1 {
		n = 1
	}
	return strings.Repeat("#", n) + strings.Repeat(".", width-n)
}

func short(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}
