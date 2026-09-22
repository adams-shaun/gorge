package main

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/internal/searchseat"
)

// resetSearchStats clears the collector and restores it after the test; the
// stats are package state, so tests must not leak into each other (or into a
// later test's report).
func resetSearchStats(t *testing.T) {
	t.Helper()
	searchStats.mu.Lock()
	saved := searchStats.diags
	searchStats.diags = nil
	searchStats.mu.Unlock()
	t.Cleanup(func() {
		searchStats.mu.Lock()
		searchStats.diags = saved
		searchStats.mu.Unlock()
	})
}

// TestSearchCostReportNumbers pins the report's arithmetic on a hand-built
// diag set: the asked/game figure, the covered fraction, the turn buckets,
// and the ms mean/p95 over the ASKED population (a fallback decision is still
// asked -- it paid the sample cost).
func TestSearchCostReportNumbers(t *testing.T) {
	resetSearchStats(t)
	searchStats.mu.Lock()
	searchStats.diags = []searchseat.Diag{
		{Turn: 3, SampleMS: 100, SearchMS: 50, Trace: searchseat.Trace{Kind: "cast", Covered: true, Attempts: 64, Accepted: 8, PrefixRejected: 56, ESS: 7.5}},
		{Turn: 4, SampleMS: 40, SearchMS: 10, Trace: searchseat.Trace{Kind: "attackers", Covered: true}},
		{Turn: 9, SampleMS: 10, SearchMS: 0, Trace: searchseat.Trace{Kind: "cast", Fallback: "sample worlds", Attempts: 64, Accepted: 2, PrefixRejected: 62, ESS: 1.2, Rejections: []searchprobe.RejectionBucket{
			{Component: "zeta", Shape: "late", Count: 2},
			{Component: "alpha", Shape: "early", Count: 4},
		}}},
		{Turn: 20, SampleMS: 20, SearchMS: 20, Trace: searchseat.Trace{Kind: "cast", Covered: true}},
	}
	searchStats.mu.Unlock()

	out := searchCostReport(4)
	for _, want := range []string{
		"asked 4 decisions over 4 games (1.0 asked/game), covered 3 (75.0%)",
		// ms totals: 150, 50, 10, 40 -> mean 62.5, p50 (sorted 10,40,50,150)
		// index int(0.5*3)=1 -> 40, p95 index int(0.95*3)=2 -> 50
		"ms/asked decision total: mean 62.5 p50 40.0 p95 50.0 (sample 42.5 + search 20.0 means)",
		"t01-06: asked 2, covered 2 (100.0%), ms mean 100.0 p95 50.0",
		"t07-12: asked 1, covered 0 (0.0%), ms mean 10.0 p95 10.0",
		"t13+: asked 1, covered 1 (100.0%), ms mean 40.0 p95 40.0",
		`fallback "sample worlds": 1`,
		"    timing: sample 70.0 + search 30.0 ms means",
		"    sampler: attempts 64, accepted 8, prefix-rejected 56; ESS weighting decisions 1, mean 7.5, p50 7.5, p95 7.5",
		"    sampler rejection \"alpha/early\": 4",
		"    sampler rejection \"zeta/late\": 2",
		"    sampler: attempts 64, accepted 2, prefix-rejected 62; ESS weighting decisions 1, mean 1.2, p50 1.2, p95 1.2",
		"    sampler: attempts 0, accepted 0, prefix-rejected 0; ESS weighting decisions 0, mean 0.0, p50 0.0, p95 0.0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
}

// TestSearchCostReportEmpty is the no-search-seat run: the report must say so
// plainly, not divide by zero.
func TestSearchCostReportEmpty(t *testing.T) {
	resetSearchStats(t)
	out := searchCostReport(7)
	if !strings.Contains(out, "no asked decisions") {
		t.Errorf("empty report = %q, want the plain no-asked-decisions line", out)
	}
}
