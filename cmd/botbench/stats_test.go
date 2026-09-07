package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// TestDecisionStatsAggregate pins the tallies behind each histogram column:
// count, mean-option-count (sumOpts), the singleton share (single counts
// len(Options)==1) and the first-option share (first counts an answer whose
// first chosen index is Options[0].Index). It also pins that write() emits
// rows in sorted-key order, never map iteration order, so the report is
// byte-deterministic for identical inputs.
func TestDecisionStatsAggregate(t *testing.T) {
	c := newDecisionStats()
	c.game()
	c.game()
	// A tap decision: two options, bot tapped the first (its chooseTap).
	c.record(&decision.Decision{Kind: decision.KPriority,
		Options: []decision.Option{{Index: 0, Kind: "activate"}, {Index: 1, Kind: "pass"}}},
		decision.Intent{Choices: []int{0}})
	// A pass decision: a single-option pass, the only legal move.
	c.record(&decision.Decision{Kind: decision.KPriority,
		Options: []decision.Option{{Index: 0, Kind: "pass"}}},
		decision.Intent{Choices: []int{0}})
	// A choose/x decision, answered with the second option (not Options[0]).
	c.record(&decision.Decision{Kind: decision.KChoose,
		Options: []decision.Option{{Index: 0, Kind: "x", Amount: 1}, {Index: 1, Kind: "x", Amount: 2}}},
		decision.Intent{Choices: []int{1}})

	c.mu.Lock()
	if c.games != 2 {
		t.Fatalf("games = %d, want 2", c.games)
	}
	if st := c.rows["priority/tap"]; st == nil || st.count != 1 || st.sumOpts != 2 || st.single != 0 || st.first != 1 {
		t.Fatalf("priority/tap row = %+v, want count 1, sumOpts 2, single 0, first 1", st)
	}
	if st := c.rows["priority/pass"]; st == nil || st.count != 1 || st.sumOpts != 1 || st.single != 1 || st.first != 1 {
		t.Fatalf("priority/pass row = %+v, want count 1, sumOpts 1, single 1, first 1", st)
	}
	if st := c.rows["choose/x"]; st == nil || st.count != 1 || st.sumOpts != 2 || st.single != 0 || st.first != 0 {
		t.Fatalf("choose/x row = %+v, want count 1, sumOpts 2, single 0, first 0 (answered second option)", st)
	}
	c.mu.Unlock()

	var buf bytes.Buffer
	c.write(&buf)
	got := buf.String()
	if !strings.Contains(got, "decision stats (2 games)") {
		t.Fatalf("write header missing: %q", got)
	}
	// Sorted-key order: choose/x < priority/pass < priority/tap.
	iChoose, iPass, iTap := strings.Index(got, "choose/x"), strings.Index(got, "priority/pass"), strings.Index(got, "priority/tap")
	if iChoose < 0 || iPass < 0 || iTap < 0 || !(iChoose < iPass && iPass < iTap) {
		t.Fatalf("rows not in sorted-key order (choose=%d pass=%d tap=%d):\n%s", iChoose, iPass, iTap, got)
	}
	if !strings.Contains(got, "0.50") || !strings.Contains(got, "1.00") {
		t.Fatalf("mean-per-game not rendered as expected:\n%s", got)
	}
}

// TestDecisionStatsConcurrent pins that the collector is safe to accumulate
// from many goroutines: the totals after a burst must equal the sum of every
// contribute, so an unguarded shared map (which the task forbids) would show
// up here as a lost count.
func TestDecisionStatsConcurrent(t *testing.T) {
	c := newDecisionStats()
	const workers, per = 40, 200
	var wg sync.WaitGroup
	for g := 0; g < workers; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.game()
			for i := 0; i < per; i++ {
				c.record(&decision.Decision{Kind: decision.KTarget,
					Options: []decision.Option{{Index: 0, Kind: "target"}}},
					decision.Intent{Choices: []int{0}})
			}
		}()
	}
	wg.Wait()
	c.mu.Lock()
	if c.games != workers {
		t.Fatalf("games = %d, want %d", c.games, workers)
	}
	if st := c.rows["target"]; st == nil || st.count != workers*per {
		t.Fatalf("target count = %d, want %d", st.count, workers*per)
	}
	c.mu.Unlock()
}

// TestPriorityBranch pins the priority sub-kind classification: the branch
// the policy took is recovered from the chosen option's Kind, so a
// priority/tap row really means the tap branch and not a mislabelled pass.
func TestPriorityBranch(t *testing.T) {
	cases := []struct {
		kind    string
		opts    []decision.Option
		choices []int
		want    string
	}{
		{"tap", []decision.Option{{Index: 0, Kind: "activate"}}, []int{0}, "tap"},
		{"land", []decision.Option{{Index: 0, Kind: "play_land"}}, []int{0}, "land"},
		{"cast", []decision.Option{{Index: 0, Kind: "cast"}}, []int{0}, "cast"},
		{"ability", []decision.Option{{Index: 0, Kind: "ability"}}, []int{0}, "ability"},
		{"pass", []decision.Option{{Index: 0, Kind: "pass"}}, []int{0}, "pass"},
		{"concede-never-picked-classified-pass", []decision.Option{{Index: 0, Kind: "concede"}}, []int{0}, "pass"},
		{"empty-choices", []decision.Option{{Index: 0, Kind: "pass"}}, nil, "other"},
		{"out-of-range", []decision.Option{{Index: 0, Kind: "pass"}}, []int{5}, "other"},
		{"unknown-kind", []decision.Option{{Index: 0, Kind: "weird"}}, []int{0}, "other"},
	}
	for _, tc := range cases {
		d := &decision.Decision{Kind: decision.KPriority, Options: tc.opts}
		if got := priorityBranch(d, decision.Intent{Choices: tc.choices}); got != tc.want {
			t.Errorf("priorityBranch(%s) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}
