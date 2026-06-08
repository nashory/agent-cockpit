package usage

import (
	"strings"
	"time"
)

// Insights holds high-level, semantic metrics derived from raw events: cost
// efficiency, spending cadence, and temporal rhythm. These are the "processed"
// readouts the cockpit surfaces beyond raw token sums.
type Insights struct {
	// Efficiency
	CacheHitRate     float64 // cache reads / (input + cache reads)
	CacheSavingsUSD  float64 // saved vs paying input rate for cached tokens
	InputOutputRatio float64 // input tokens per output token
	ReasoningShare   float64 // reasoning tokens / total tokens
	EffectiveRateUSD float64 // total cost per 1M output tokens

	// Cadence
	Sessions             int
	ActiveDays           int
	SpanDays             int
	SessionsPerActiveDay float64
	AvgTokensPerSession  float64
	AvgCostPerSession    float64

	// Economics
	TotalCostUSD        float64
	CostPerActiveDay    float64
	ProjectedMonthlyUSD float64 // calendar-span run-rate × 30
	PremiumCostShare    float64 // opus-tier cost / total cost

	// Temporal rhythm (tokens)
	HourHist       [24]int64
	WeekdayHist    [7]int64
	BusiestHour    int
	BusiestWeekday int

	// Diversity
	Projects int
	Models   int
	Agents   int
}

// ComputeInsights folds a slice of events into derived metrics.
func ComputeInsights(events []Event, prices PriceBook) Insights {
	var ins Insights
	if len(events) == 0 {
		return ins
	}

	var totalCost, cacheSavings, opusCost float64
	var totalTok, totalIn, totalOut, totalCacheRead, totalReasoning int64
	sessions := map[string]struct{}{}
	projects := map[string]struct{}{}
	models := map[string]struct{}{}
	agentSet := map[string]struct{}{}
	days := map[string]struct{}{}
	var minT, maxT time.Time

	for _, e := range events {
		c := EstimateCostWith(e, prices)
		totalCost += c
		totalTok += e.TotalTokens()
		totalIn += e.Input
		totalOut += e.Output
		totalCacheRead += e.CacheRead
		totalReasoning += e.Reasoning

		if e.SessionID != "" {
			sessions[e.SessionID] = struct{}{}
		}
		if e.Project != "" {
			projects[e.Project] = struct{}{}
		}
		if e.Model != "" {
			models[e.Model] = struct{}{}
		}
		if e.Source != "" {
			agentSet[e.Source] = struct{}{}
		}
		if !e.Timestamp.IsZero() {
			days[e.Timestamp.Format("2006-01-02")] = struct{}{}
			if minT.IsZero() || e.Timestamp.Before(minT) {
				minT = e.Timestamp
			}
			if e.Timestamp.After(maxT) {
				maxT = e.Timestamp
			}
			ins.HourHist[e.Timestamp.Hour()] += e.TotalTokens()
			ins.WeekdayHist[int(e.Timestamp.Weekday())] += e.TotalTokens()
		}
		if strings.Contains(strings.ToLower(e.Model), "opus") {
			opusCost += c
		}
		p := lookupPricing(e.Model, prices)
		if p.InputPerMillion > p.CacheReadPerMillion {
			cacheSavings += float64(e.CacheRead) * (p.InputPerMillion - p.CacheReadPerMillion) / 1_000_000
		}
	}

	ins.TotalCostUSD = totalCost
	ins.Sessions = len(sessions)
	ins.Projects = len(projects)
	ins.Models = len(models)
	ins.Agents = len(agentSet)
	ins.ActiveDays = len(days)
	if !minT.IsZero() {
		ins.SpanDays = int(maxT.Truncate(24*time.Hour).Sub(minT.Truncate(24*time.Hour))/(24*time.Hour)) + 1
	}

	if totalIn+totalCacheRead > 0 {
		ins.CacheHitRate = float64(totalCacheRead) / float64(totalIn+totalCacheRead)
	}
	ins.CacheSavingsUSD = cacheSavings
	if totalOut > 0 {
		ins.InputOutputRatio = float64(totalIn) / float64(totalOut)
		ins.EffectiveRateUSD = totalCost / float64(totalOut) * 1_000_000
	}
	if totalTok > 0 {
		ins.ReasoningShare = float64(totalReasoning) / float64(totalTok)
	}
	if ins.Sessions > 0 {
		ins.AvgTokensPerSession = float64(totalTok) / float64(ins.Sessions)
		ins.AvgCostPerSession = totalCost / float64(ins.Sessions)
	}
	if ins.ActiveDays > 0 {
		ins.CostPerActiveDay = totalCost / float64(ins.ActiveDays)
		ins.SessionsPerActiveDay = float64(ins.Sessions) / float64(ins.ActiveDays)
	}
	if ins.SpanDays > 0 {
		ins.ProjectedMonthlyUSD = totalCost / float64(ins.SpanDays) * 30
	}
	if totalCost > 0 {
		ins.PremiumCostShare = opusCost / totalCost
	}

	// -1 means "no timestamped activity" so callers can show a blank instead of
	// claiming midnight Sunday is the busiest slot.
	ins.BusiestHour, ins.BusiestWeekday = -1, -1
	var maxHour, maxWeekday int64
	for h := 0; h < 24; h++ {
		if ins.HourHist[h] > maxHour {
			maxHour = ins.HourHist[h]
			ins.BusiestHour = h
		}
	}
	for d := 0; d < 7; d++ {
		if ins.WeekdayHist[d] > maxWeekday {
			maxWeekday = ins.WeekdayHist[d]
			ins.BusiestWeekday = d
		}
	}
	return ins
}
