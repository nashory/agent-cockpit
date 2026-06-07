package usage

import (
	"sort"
	"strings"
	"time"
)

type Event struct {
	Source      string    `json:"source"`
	SessionID   string    `json:"session_id,omitempty"`
	Project     string    `json:"project,omitempty"`
	CWD         string    `json:"cwd,omitempty"`
	Model       string    `json:"model,omitempty"`
	Input       int64     `json:"input_tokens"`
	Output      int64     `json:"output_tokens"`
	CacheRead   int64     `json:"cache_read_tokens,omitempty"`
	CacheCreate int64     `json:"cache_create_tokens,omitempty"`
	Reasoning   int64     `json:"reasoning_tokens,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

func (e Event) TotalTokens() int64 {
	return e.Input + e.Output + e.CacheRead + e.CacheCreate + e.Reasoning
}

type Pricing struct {
	InputPerMillion      float64 `toml:"input_per_million" json:"input_per_million"`
	OutputPerMillion     float64 `toml:"output_per_million" json:"output_per_million"`
	CacheReadPerMillion  float64 `toml:"cache_read_per_million" json:"cache_read_per_million"`
	CacheWritePerMillion float64 `toml:"cache_write_per_million" json:"cache_write_per_million"`
}

type PriceBook map[string]Pricing

func DefaultPricing(model string) Pricing {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return Pricing{InputPerMillion: 15, OutputPerMillion: 75, CacheReadPerMillion: 1.50, CacheWritePerMillion: 18.75}
	case strings.Contains(m, "sonnet"):
		return Pricing{InputPerMillion: 3, OutputPerMillion: 15, CacheReadPerMillion: 0.30, CacheWritePerMillion: 3.75}
	case strings.Contains(m, "haiku"):
		return Pricing{InputPerMillion: 0.80, OutputPerMillion: 4, CacheReadPerMillion: 0.08, CacheWritePerMillion: 1}
	case strings.Contains(m, "gpt-5"):
		return Pricing{InputPerMillion: 1.25, OutputPerMillion: 10, CacheReadPerMillion: 0.125}
	case strings.Contains(m, "gpt-4.1"):
		return Pricing{InputPerMillion: 2, OutputPerMillion: 8, CacheReadPerMillion: 0.50}
	case strings.Contains(m, "gemini-3"):
		return Pricing{InputPerMillion: 2, OutputPerMillion: 12}
	case strings.Contains(m, "gemini-2.5-pro"):
		return Pricing{InputPerMillion: 1.25, OutputPerMillion: 10}
	case strings.Contains(m, "gemini-2.5-flash"):
		return Pricing{InputPerMillion: 0.30, OutputPerMillion: 2.50}
	default:
		return Pricing{}
	}
}

func EstimateCost(e Event) float64 {
	return EstimateCostWith(e, nil)
}

func EstimateCostWith(e Event, prices PriceBook) float64 {
	p := lookupPricing(e.Model, prices)
	return (float64(e.Input)*p.InputPerMillion +
		float64(e.Output)*p.OutputPerMillion +
		float64(e.CacheRead)*p.CacheReadPerMillion +
		float64(e.CacheCreate)*p.CacheWritePerMillion) / 1_000_000
}

func lookupPricing(model string, prices PriceBook) Pricing {
	model = strings.ToLower(model)
	var bestKey string
	var best Pricing
	for key, price := range prices {
		k := strings.ToLower(key)
		if k != "" && strings.Contains(model, k) && len(k) > len(bestKey) {
			bestKey = k
			best = price
		}
	}
	if bestKey != "" {
		return best
	}
	return DefaultPricing(model)
}

type Totals struct {
	Events      int     `json:"events"`
	Input       int64   `json:"input_tokens"`
	Output      int64   `json:"output_tokens"`
	CacheRead   int64   `json:"cache_read_tokens"`
	CacheCreate int64   `json:"cache_create_tokens"`
	Reasoning   int64   `json:"reasoning_tokens"`
	Total       int64   `json:"total_tokens"`
	CostUSD     float64 `json:"estimated_cost_usd"`
}

func Summarize(events []Event) Totals {
	return SummarizeWith(events, nil)
}

func SummarizeWith(events []Event, prices PriceBook) Totals {
	var t Totals
	for _, e := range events {
		t.Events++
		t.Input += e.Input
		t.Output += e.Output
		t.CacheRead += e.CacheRead
		t.CacheCreate += e.CacheCreate
		t.Reasoning += e.Reasoning
		t.Total += e.TotalTokens()
		t.CostUSD += EstimateCostWith(e, prices)
	}
	return t
}

type Bucket struct {
	Key    string  `json:"key"`
	Totals Totals  `json:"totals"`
	Share  float64 `json:"share"`
}

func GroupBy(events []Event, key func(Event) string) []Bucket {
	return GroupByWith(events, nil, key)
}

func GroupByWith(events []Event, prices PriceBook, key func(Event) string) []Bucket {
	total := SummarizeWith(events, prices).Total
	byKey := map[string][]Event{}
	for _, e := range events {
		k := key(e)
		if k == "" {
			k = "unknown"
		}
		byKey[k] = append(byKey[k], e)
	}
	buckets := make([]Bucket, 0, len(byKey))
	for k, evs := range byKey {
		t := SummarizeWith(evs, prices)
		share := 0.0
		if total > 0 {
			share = float64(t.Total) / float64(total)
		}
		buckets = append(buckets, Bucket{Key: k, Totals: t, Share: share})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Totals.Total > buckets[j].Totals.Total
	})
	return buckets
}

func Filter(events []Event, since, until time.Time, sources []string, project, model string) []Event {
	sourceSet := map[string]bool{}
	for _, s := range sources {
		if s != "" {
			sourceSet[strings.ToLower(s)] = true
		}
	}
	project = strings.ToLower(project)
	model = strings.ToLower(model)

	out := events[:0]
	for _, e := range events {
		if !since.IsZero() && e.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && e.Timestamp.After(until) {
			continue
		}
		if len(sourceSet) > 0 && !sourceSet[strings.ToLower(e.Source)] {
			continue
		}
		if project != "" && !strings.Contains(strings.ToLower(e.Project+" "+e.CWD), project) {
			continue
		}
		if model != "" && !strings.Contains(strings.ToLower(e.Model), model) {
			continue
		}
		out = append(out, e)
	}
	return out
}
