package tui

import (
	"testing"
	"time"

	"github.com/nashory/agent-cockpit/internal/usage"
)

func TestStackBandsSumToHeight(t *testing.T) {
	cases := []struct {
		daily [][]int64
		total int64
		h     int
	}{
		{[][]int64{{50}, {50}}, 100, 8},
		{[][]int64{{70}, {30}}, 100, 8},
		{[][]int64{{1}, {1}, {1}}, 3, 10},
		{[][]int64{{999}, {1}}, 1000, 7},
	}
	for _, c := range cases {
		rows := stackBands(c.daily, 0, c.total, c.h)
		sum := 0
		for _, r := range rows {
			if r < 0 {
				t.Fatalf("negative band rows: %v", rows)
			}
			sum += r
		}
		if sum != c.h {
			t.Fatalf("bands %v sum to %d, want H=%d", rows, sum, c.h)
		}
	}
	// 50/50 over height 8 splits evenly.
	if r := stackBands([][]int64{{50}, {50}}, 0, 100, 8); r[0] != 4 || r[1] != 4 {
		t.Fatalf("50/50 -> %v, want [4 4]", r)
	}
	// 70/30 gives the larger share more rows.
	if r := stackBands([][]int64{{70}, {30}}, 0, 100, 8); r[0] <= r[1] {
		t.Fatalf("70/30 -> %v, want first band larger", r)
	}
}

func TestModelMixTopNAndOther(t *testing.T) {
	now := time.Now()
	var evs []usage.Event
	// 7 models with descending totals; folded into top-5 + other.
	for i, n := range []int64{700, 600, 500, 400, 300, 200, 100} {
		evs = append(evs, usage.Event{
			Model:     string(rune('a' + i)),
			Output:    n,
			Timestamp: now.AddDate(0, 0, -i%3),
		})
	}
	labels, colors, daily, totals, grand := modelMix(evs, 7)

	if grand != 700+600+500+400+300+200+100 {
		t.Fatalf("grand = %d, want 2800", grand)
	}
	// top 5 distinct + "other".
	if len(labels) != stackTopModels+1 || labels[len(labels)-1] != "other" {
		t.Fatalf("labels = %v, want 5 models + other", labels)
	}
	if len(colors) != len(labels) || len(daily) != len(labels) || len(totals) != len(labels) {
		t.Fatal("parallel slices length mismatch")
	}
	// ranked descending for the top models.
	for i := 1; i < stackTopModels; i++ {
		if totals[i] > totals[i-1] {
			t.Fatalf("totals not descending: %v", totals)
		}
	}
	// "other" = 200 + 100.
	if totals[len(totals)-1] != 300 {
		t.Fatalf("other total = %d, want 300", totals[len(totals)-1])
	}
}
