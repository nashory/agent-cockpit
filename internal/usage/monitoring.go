package usage

import "time"

type Budget struct {
	DailyUSD    float64 `toml:"daily_usd" json:"daily_usd"`
	WeeklyUSD   float64 `toml:"weekly_usd" json:"weekly_usd"`
	MonthlyUSD  float64 `toml:"monthly_usd" json:"monthly_usd"`
	WarnPct     float64 `toml:"warn_pct" json:"warn_pct"`
	CriticalPct float64 `toml:"critical_pct" json:"critical_pct"`
}

type Limits struct {
	Claude5HTokens int64   `toml:"claude_5h_tokens" json:"claude_5h_tokens"`
	Claude7DTokens int64   `toml:"claude_7d_tokens" json:"claude_7d_tokens"`
	WarnPct        float64 `toml:"warn_pct" json:"warn_pct"`
	CriticalPct    float64 `toml:"critical_pct" json:"critical_pct"`
}

type ThresholdStatus struct {
	Name      string    `json:"name"`
	Used      float64   `json:"used"`
	Limit     float64   `json:"limit"`
	Ratio     float64   `json:"ratio"`
	Level     string    `json:"level"`
	ResetTime time.Time `json:"reset_time,omitempty"`
}

func thresholdLevel(ratio, warnPct, criticalPct float64) string {
	if warnPct <= 0 {
		warnPct = 80
	}
	if criticalPct <= 0 {
		criticalPct = 95
	}
	warn := warnPct / 100
	critical := criticalPct / 100
	switch {
	case ratio >= critical:
		return "critical"
	case ratio >= warn:
		return "warn"
	default:
		return "ok"
	}
}

func status(name string, used, limit, warnPct, criticalPct float64, reset time.Time) ThresholdStatus {
	var ratio float64
	if limit > 0 {
		ratio = used / limit
	}
	return ThresholdStatus{
		Name:      name,
		Used:      used,
		Limit:     limit,
		Ratio:     ratio,
		Level:     thresholdLevel(ratio, warnPct, criticalPct),
		ResetTime: reset,
	}
}

func BudgetStatuses(events []Event, prices PriceBook, b Budget, now time.Time) []ThresholdStatus {
	if now.IsZero() {
		now = time.Now()
	}
	var out []ThresholdStatus
	if b.DailyUSD > 0 {
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		reset := start.AddDate(0, 0, 1)
		out = append(out, status("daily budget", SummarizeWith(Filter(events, start, time.Time{}, nil, "", ""), prices).CostUSD, b.DailyUSD, b.WarnPct, b.CriticalPct, reset))
	}
	if b.WeeklyUSD > 0 {
		start := now.AddDate(0, 0, -6)
		reset := now.AddDate(0, 0, 1)
		out = append(out, status("7d budget", SummarizeWith(Filter(events, start, time.Time{}, nil, "", ""), prices).CostUSD, b.WeeklyUSD, b.WarnPct, b.CriticalPct, reset))
	}
	if b.MonthlyUSD > 0 {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		reset := start.AddDate(0, 1, 0)
		out = append(out, status("monthly budget", SummarizeWith(Filter(events, start, time.Time{}, nil, "", ""), prices).CostUSD, b.MonthlyUSD, b.WarnPct, b.CriticalPct, reset))
	}
	return out
}

func ClaudeLimitStatuses(events []Event, prices PriceBook, l Limits, now time.Time) []ThresholdStatus {
	if now.IsZero() {
		now = time.Now()
	}
	var claude []Event
	for _, e := range events {
		if e.Source == "claude" {
			claude = append(claude, e)
		}
	}
	var out []ThresholdStatus
	if l.Claude5HTokens > 0 {
		var active *Block
		blocks := sessionBlocksAt(claude, prices, DefaultBlockWindow, now)
		for i := range blocks {
			if blocks[i].Active {
				active = &blocks[i]
				break
			}
		}
		if active != nil {
			out = append(out, status("claude 5h", float64(active.Totals.Total), float64(l.Claude5HTokens), l.WarnPct, l.CriticalPct, active.End))
		} else {
			out = append(out, status("claude 5h", 0, float64(l.Claude5HTokens), l.WarnPct, l.CriticalPct, time.Time{}))
		}
	}
	if l.Claude7DTokens > 0 {
		start := now.AddDate(0, 0, -7)
		t := SummarizeWith(Filter(claude, start, time.Time{}, nil, "", ""), prices)
		out = append(out, status("claude 7d", float64(t.Total), float64(l.Claude7DTokens), l.WarnPct, l.CriticalPct, now.AddDate(0, 0, 1)))
	}
	return out
}

func WorstStatus(statuses []ThresholdStatus) ThresholdStatus {
	var worst ThresholdStatus
	for _, s := range statuses {
		if severity(s.Level) > severity(worst.Level) || (severity(s.Level) == severity(worst.Level) && s.Ratio > worst.Ratio) {
			worst = s
		}
	}
	return worst
}

func severity(level string) int {
	switch level {
	case "critical":
		return 3
	case "warn":
		return 2
	case "ok":
		return 1
	default:
		return 0
	}
}
