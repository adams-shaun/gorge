package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
)

// TestBlankLineIsASilentNoOp pins the display spacer's contract: DB$ BlankLine
// resolves to nothing at all. It is registered so the resolver does not emit
// its generic "unimplemented API BlankLine" Note (the Path cycle's DBSpace
// SVar is one), and it must stay SILENT -- a Note would put noise in every
// log that walks through it.
func TestBlankLineIsASilentNoOp(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Source: 0}, sa(t, "SP$ BlankLine"))
	if len(h.log) != 0 {
		t.Fatalf("BlankLine emitted %d events, want none: %+v", len(h.log), h.log)
	}
}

// TestPlaneswalkRecordsTheNoPlanarDeckDegrade pins CR 901.8's degrade: with no
// planar deck the planeswalk resolves as a recorded no-op (there is no next
// plane and no planeswalk-away trigger to fire).
func TestPlaneswalkRecordsTheNoPlanarDeckDegrade(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Source: 0}, sa(t, "SP$ Planeswalk"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("Planeswalk log = %+v, want one Note", h.log)
	}
	if h.log[0].Text != "planeswalk (no planar deck)" {
		t.Fatalf("Planeswalk note = %q", h.log[0].Text)
	}
}

// TestChaosEnsuesRecordsTheNoPlanarDeckDegrade pins CR 901.9's degrade the
// same way.
func TestChaosEnsuesRecordsTheNoPlanarDeckDegrade(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Source: 0}, sa(t, "SP$ ChaosEnsues"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("ChaosEnsues log = %+v, want one Note", h.log)
	}
	if h.log[0].Text != "chaos ensues (no planar deck)" {
		t.Fatalf("ChaosEnsues note = %q", h.log[0].Text)
	}
}

// TestPlanechaseVerbsNeverEmitUnimplemented is the regression guard for the
// gap the brief named: a registered verb must not reach Resolve's
// "unimplemented API" fallback.
func TestPlanechaseVerbsNeverEmitUnimplemented(t *testing.T) {
	for _, api := range []string{"BlankLine", "Planeswalk", "ChaosEnsues"} {
		if !Supported()["api:"+api] {
			t.Fatalf("api:%s is not registered", api)
		}
		h := newHost(t, 2)
		Resolve(h, &Ctx{Source: 0}, sa(t, "SP$ "+api))
		for _, e := range h.log {
			if e.Kind == events.Note && strings.HasPrefix(e.Text, "unimplemented API") {
				t.Fatalf("%s emitted %q", api, e.Text)
			}
		}
	}
}

// voteFixtureSA builds a Vote SA with real SVar bodies so the fixed-list
// outcome resolution can run a named sub. The chosen SVar's body is a
// Registered probe, so the test observes the winner without any corpus text.
func voteFixtureSA(t *testing.T, line string) (*fakeHost, *cards.SA, map[string]string, *[]string) {
	t.Helper()
	var order []string
	Register("TestVoteA", func(_ Host, _ *Ctx, _ *cards.SA) { order = append(order, "A") })
	Register("TestVoteB", func(_ Host, _ *Ctx, _ *cards.SA) { order = append(order, "B") })
	t.Cleanup(func() { unregister("TestVoteA", "TestVoteB") })
	c, _ := cards.ParseBytes("v.txt", []byte("Name:T\nTypes:Sorcery\nA:"+line+"\nOracle:x\n"))
	c.Link()
	h := newHost(t, 2)
	return h, c.Faces[0].Abilities[0], c.Faces[0].SVars, &order
}

// TestVoteFixedListRunsTheWinningOutcome is the core of the brief's premise
// correction: the fixed-list Vote used to resolve NOTHING, so the Path
// cycle's DBPlaneswalk/DBChaos never ran and the registered planechase verbs
// were unreachable from a vote. Every voter takes the first option (the
// deterministic stand-in), so the first SVar wins and runs.
func TestVoteFixedListRunsTheWinningOutcome(t *testing.T) {
	h, s, svars, order := voteFixtureSA(t, "SP$ Vote | Defined$ Player | Choices$ AChoice,BChoice "+
		"| VoteTiedAbility$ BChoice\nSVar:AChoice:DB$ TestVoteA\nSVar:BChoice:DB$ TestVoteB\n")
	Resolve(h, &Ctx{Source: 0, SVars: svars}, s)
	if len(*order) != 1 || (*order)[0] != "A" {
		t.Fatalf("winning outcome ran %v, want exactly [A]", *order)
	}
	// One Note per voting player, naming the option voted for.
	notes := 0
	for _, e := range h.log {
		if e.Kind == events.Note && e.Text == "votes for AChoice" {
			notes++
		}
	}
	if notes != 2 {
		t.Fatalf("%d vote Notes, want 2", notes)
	}
}

// TestVoteWinnerPicksTheHighestAndReportsTies exercises the tally directly.
// The deterministic stand-in cannot produce a live tie today (every vote goes
// to option 0), so a live resolution never enters the tie branch; the
// real-SA tie path is pinned by TestVoteAnsweredTieRunsVoteTiedAbility
// (effects) and TestPathOfTheAnimistTiedVoteRunsTheTiedBranch (rules). The
// first highest index wins the tie, matching the oracle's "if planeswalk
// gets more votes" over "or tied".
func TestVoteWinnerPicksTheHighestAndReportsTies(t *testing.T) {
	cases := []struct {
		counts []int
		want   int
		tied   bool
	}{
		{[]int{2, 0}, 0, false},
		{[]int{0, 3, 1}, 1, false},
		{[]int{1, 1}, 0, true},
		{[]int{1, 0, 1}, 0, true},
		{[]int{0, 0, 2}, 2, false},
		{[]int{0, 0}, 0, true},
		{nil, 0, false},
	}
	for _, tc := range cases {
		got, tied := voteWinner(tc.counts)
		if got != tc.want || tied != tc.tied {
			t.Fatalf("voteWinner(%v) = (%d,%v), want (%d,%v)", tc.counts, got, tied, tc.want, tc.tied)
		}
	}
}

// TestVoteAnsweredTieRunsVoteTiedAbility pins effVote's tie branch through
// Ctx.Votes, the answered per-voter choice list (the seam a real per-player
// ask fills). The stand-in gives every voter option 0, so a live resolution
// can never tie and this branch would otherwise be unreachable from any SA.
// The answered tally is consumed and cleared (fx42), and its Notes name the
// actual answered choices rather than the stand-in's option 0.
func TestVoteAnsweredTieRunsVoteTiedAbility(t *testing.T) {
	h, s, svars, order := voteFixtureSA(t, "SP$ Vote | Defined$ Player | Choices$ AChoice,BChoice "+
		"| VoteTiedAbility$ BChoice\nSVar:AChoice:DB$ TestVoteA\nSVar:BChoice:DB$ TestVoteB\n")
	c := &Ctx{Source: 0, SVars: svars, Votes: []int{0, 1}}
	Resolve(h, c, s)
	if len(*order) != 1 || (*order)[0] != "B" {
		t.Fatalf("tied vote ran %v, want exactly [B] (VoteTiedAbility$)", *order)
	}
	if c.Votes != nil {
		t.Fatalf("Ctx.Votes survived the resolution (%v), want consumed and cleared", c.Votes)
	}
	labels := map[string]int{}
	for _, e := range h.log {
		if e.Kind == events.Note && strings.HasPrefix(e.Text, "votes for ") {
			labels[e.Text]++
		}
	}
	if labels["votes for AChoice"] != 1 || labels["votes for BChoice"] != 1 {
		t.Fatalf("vote Notes = %v, want one for each answered choice", labels)
	}
}
