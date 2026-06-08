package usage

import (
	"sort"
	"time"
)

// DefaultBlockWindow is the rolling activity window. It mirrors Claude Code's
// 5-hour rate-limit / billing window so the active block shows how much of the
// current window you have already spent.
const DefaultBlockWindow = 5 * time.Hour

// Block is one activity window: a span that starts at the first event (floored
// to the hour) and lasts Window. Consecutive events fall in the same block
// until one lands at or after the window end, which opens a new block.
type Block struct {
	Start        time.Time
	End          time.Time // Start + window
	LastActivity time.Time
	Totals       Totals
	Models       map[string]struct{}
	Active       bool // the window currently containing "now"
}

// SessionBlocks groups events into activity windows using time.Now() to mark the
// active block. Blocks are returned oldest first.
func SessionBlocks(events []Event, prices PriceBook, window time.Duration) []Block {
	return sessionBlocksAt(events, prices, window, time.Now())
}

func sessionBlocksAt(events []Event, prices PriceBook, window time.Duration, now time.Time) []Block {
	if window <= 0 {
		window = DefaultBlockWindow
	}
	ts := make([]Event, 0, len(events))
	for _, e := range events {
		if !e.Timestamp.IsZero() {
			ts = append(ts, e)
		}
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i].Timestamp.Before(ts[j].Timestamp) })

	var blocks []Block
	var cur *Block
	for _, e := range ts {
		if cur == nil || !e.Timestamp.Before(cur.End) {
			start := e.Timestamp.Truncate(time.Hour)
			blocks = append(blocks, Block{
				Start:  start,
				End:    start.Add(window),
				Models: map[string]struct{}{},
			})
			cur = &blocks[len(blocks)-1]
		}
		cur.Totals.Events++
		cur.Totals.Input += e.Input
		cur.Totals.Output += e.Output
		cur.Totals.CacheRead += e.CacheRead
		cur.Totals.CacheCreate += e.CacheCreate
		cur.Totals.Reasoning += e.Reasoning
		cur.Totals.Total += e.TotalTokens()
		cur.Totals.CostUSD += EstimateCostWith(e, prices)
		cur.LastActivity = e.Timestamp
		if e.Model != "" {
			cur.Models[e.Model] = struct{}{}
		}
	}
	for i := range blocks {
		if !now.Before(blocks[i].Start) && now.Before(blocks[i].End) {
			blocks[i].Active = true
		}
	}
	return blocks
}
