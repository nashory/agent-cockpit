package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nashory/agent-cockpit/internal/usage"
)

func Overview(w io.Writer, title string, events []usage.Event) {
	t := usage.Summarize(events)
	fmt.Fprintf(w, "%s\n\n", title)
	fmt.Fprintf(w, "Events: %d\n", t.Events)
	fmt.Fprintf(w, "Tokens: %s total  %s input  %s output  %s cache  %s reasoning\n",
		formatInt(t.Total), formatInt(t.Input), formatInt(t.Output), formatInt(t.CacheRead+t.CacheCreate), formatInt(t.Reasoning))
	fmt.Fprintf(w, "Estimated cost: $%.2f\n\n", t.CostUSD)
	Buckets(w, "Agents", usage.GroupBy(events, func(e usage.Event) string { return e.Source }), 8)
	fmt.Fprintln(w)
	Buckets(w, "Models", usage.GroupBy(events, func(e usage.Event) string { return e.Model }), 8)
}

func Buckets(w io.Writer, title string, buckets []usage.Bucket, limit int) {
	fmt.Fprintln(w, title)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Name\tShare\tTokens\tCost")
	for i, b := range buckets {
		if limit > 0 && i >= limit {
			break
		}
		fmt.Fprintf(tw, "%s\t%.1f%%\t%s\t$%.2f\n", b.Key, b.Share*100, formatInt(b.Totals.Total), b.Totals.CostUSD)
	}
	_ = tw.Flush()
}

func Sessions(w io.Writer, events []usage.Event, limit int) {
	buckets := usage.GroupBy(events, func(e usage.Event) string {
		if e.Project != "" && e.SessionID != "" {
			return e.Project + " / " + short(e.SessionID)
		}
		return e.SessionID
	})
	Buckets(w, "Sessions", buckets, limit)
}

func Trend(w io.Writer, events []usage.Event, days int) {
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
		totals[i] = usage.Summarize(byDay[i])
		if totals[i].Total > max {
			max = totals[i].Total
		}
	}
	for i, t := range totals {
		day := start.AddDate(0, 0, i).Format("Jan 02")
		fmt.Fprintf(w, "%s  %-24s %s  $%.2f\n", day, bar(t.Total, max, 24), formatInt(t.Total), t.CostUSD)
	}
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
