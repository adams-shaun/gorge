package main

// The search seat's cost report (the teacher-as-seat gate's "Done means"):
// mean and p95 wall-clock per ASKED decision, asked decisions per game, the
// covered fraction, all broken out by turn bucket. The clocks live here --
// cmd/botbench is one of the packages internal/archtest allows to import
// time -- and reach the seat through searchseat.Millis (the timing source)
// and searchseat.Watch (the per-asked-decision diagnostic), both installed
// once by mainExit before any game starts and never touched afterwards.
// Games run on several workers, so the collector takes a mutex; nothing it
// records reaches a game, an event, a view or a replay -- a report is bench
// output, not engine state, which is the same reason the profiler and the
// grind deadline are allowed their clocks.

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/internal/searchseat"
)

// searchKnobs is the "search" policy's Options: initialized to the teacher's
// measured defaults (searchseat.Defaults, the knobs the +5.80pp +/- 1.25
// paired-dev result was taken with) and overwritten once by main() from the
// -search-* flags. Write-once before any game starts, read-only afterwards --
// the same pattern as castProfileOverride and policynetModel.
var searchKnobs = searchseat.Defaults()

// searchStats collects the Watch diags; guarded, because the pool plays
// several games at once.
var searchStats struct {
	mu    sync.Mutex
	diags []searchseat.Diag
}

// installSearchCostStats wires the search seat's timing and diagnostic hooks
// to this command's collector. Called once from mainExit before any game
// starts; a run without a search side never calls it, so the hooks stay nil
// and the seat is untimed and silent.
func installSearchCostStats() {
	t0 := time.Now()
	searchseat.Millis = func() float64 {
		return float64(time.Since(t0).Microseconds()) / 1000
	}
	searchseat.Watch = func(dg searchseat.Diag) {
		searchStats.mu.Lock()
		searchStats.diags = append(searchStats.diags, dg)
		searchStats.mu.Unlock()
	}
}

// searchCostReport renders the cost report over the collected diags.
// totalGames is the run's game count (pairs x games in matrix mode), the
// denominator of the per-game figure.
func searchCostReport(totalGames int) string {
	searchStats.mu.Lock()
	diags := append([]searchseat.Diag(nil), searchStats.diags...)
	searchStats.mu.Unlock()

	var b strings.Builder
	if len(diags) == 0 {
		fmt.Fprintf(&b, "search cost report: no asked decisions (no search seat ran, or none was eligible)\n")
		return b.String()
	}

	var total, sampleMS, searchMS []float64
	var covered int
	type bucket struct {
		asked, covered                     int
		attempts, accepted, prefixRejected int
		ms, sampleMS, searchMS             []float64
		ess                                []float64
		rejections                         map[string]int
	}
	buckets := map[string]*bucket{}
	for _, b := range []string{"t01-06", "t07-12", "t13+"} {
		buckets[b] = &bucket{rejections: make(map[string]int)}
	}
	bucketOf := func(turn int32) string {
		switch {
		case turn > 12:
			return "t13+"
		case turn > 6:
			return "t07-12"
		default:
			return "t01-06"
		}
	}
	var fallbacks map[string]int
	for _, dg := range diags {
		ms := dg.SampleMS + dg.SearchMS
		total = append(total, ms)
		sampleMS = append(sampleMS, dg.SampleMS)
		searchMS = append(searchMS, dg.SearchMS)
		bk := buckets[bucketOf(dg.Turn)]
		bk.asked++
		bk.ms = append(bk.ms, ms)
		bk.sampleMS = append(bk.sampleMS, dg.SampleMS)
		bk.searchMS = append(bk.searchMS, dg.SearchMS)
		bk.attempts += dg.Trace.Attempts
		bk.accepted += dg.Trace.Accepted
		bk.prefixRejected += dg.Trace.PrefixRejected
		// ESS is meaningful only after at least one proposal reached the
		// weighting phase; failed attempts have the zero-value ESS.
		if dg.Trace.Accepted > 0 {
			bk.ess = append(bk.ess, dg.Trace.ESS)
		}
		for _, r := range dg.Trace.Rejections {
			bk.rejections[r.Component+"/"+r.Shape] += r.Count
		}
		if dg.Trace.Covered {
			covered++
			bk.covered++
		} else {
			if fallbacks == nil {
				fallbacks = map[string]int{}
			}
			fallbacks[dg.Trace.Fallback]++
		}
	}

	fmt.Fprintf(&b, "search cost report: asked %d decisions over %d games (%.1f asked/game), covered %d (%.1f%%)\n",
		len(diags), totalGames, float64(len(diags))/float64(totalGames), covered, 100*float64(covered)/float64(len(diags)))
	fmt.Fprintf(&b, "ms/asked decision total: mean %.1f p50 %.1f p95 %.1f (sample %.1f + search %.1f means)\n",
		meanF(total), quantF(total, .5), quantF(total, .95), meanF(sampleMS), meanF(searchMS))
	for _, name := range []string{"t01-06", "t07-12", "t13+"} {
		bk := buckets[name]
		if bk.asked == 0 {
			fmt.Fprintf(&b, "  %s: no asked decisions\n", name)
			continue
		}
		fmt.Fprintf(&b, "  %s: asked %d, covered %d (%.1f%%), ms mean %.1f p95 %.1f\n",
			name, bk.asked, bk.covered, 100*float64(bk.covered)/float64(bk.asked), meanF(bk.ms), quantF(bk.ms, .95))
		fmt.Fprintf(&b, "    timing: sample %.1f + search %.1f ms means\n", meanF(bk.sampleMS), meanF(bk.searchMS))
		fmt.Fprintf(&b, "    sampler: attempts %d, accepted %d, prefix-rejected %d; ESS weighting decisions %d, mean %.1f, p50 %.1f, p95 %.1f\n",
			bk.attempts, bk.accepted, bk.prefixRejected, len(bk.ess), meanF(bk.ess), quantF(bk.ess, .5), quantF(bk.ess, .95))
		var shapes []string
		for shape := range bk.rejections {
			shapes = append(shapes, shape)
		}
		sort.Strings(shapes)
		for _, shape := range shapes {
			fmt.Fprintf(&b, "    sampler rejection %q: %d\n", shape, bk.rejections[shape])
		}
	}
	var fk []string
	for k := range fallbacks {
		fk = append(fk, k)
	}
	sort.Strings(fk)
	for _, k := range fk {
		fmt.Fprintf(&b, "  fallback %q: %d\n", k, fallbacks[k])
	}
	return b.String()
}

func meanF(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// quantF is cmd/searchteacher's quant: nearest-rank over a sorted copy.
func quantF(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	return s[int(q*float64(len(s)-1))]
}
