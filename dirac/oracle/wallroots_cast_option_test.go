package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestWallOfRootsCastOptionAfterResolve replays the dr-mbc-bb pair whose
// divergence this fix is for (recorded before MBC was split by hash; the
// table is built from the recording's own decks, pool "custom"): after Wall of Roots resolved (its {G} a
// counter-cost gap), a priority decision for seat 0 listed gorge's
// cast(Drumhunter {3}{G}) and cast(Irregular Cohort {2}{W}{W}) -- both need
// Peat Bog's depletion rider ({T}, remove a depletion counter: Add {B}{B})
// beside the Wall's {G} -- while the kernel listed only pass and the land
// play. scriptOffers' tapX branch and tapXPrefix swallowed the board (the
// tapX search's gap set has no rider step); both now yield to the
// scriptDeadEndAny route when a rider gap is live (CR 605.3b).
//
// The replay tail is FirstPolicy for 200 further decisions, so a fix that
// merely moves the disagreement later still fails.
func TestWallOfRootsCastOptionAfterResolve(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", "tapsdefer_rider_beside_counter_cost_gap.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "custom" {
		t.Fatalf("precondition: recorded on pool %q, want custom (a named MBC split is not stable)", r.Pool)
	}
	set := reproSet(t, r.Pool, r)
	if r.Divergence == nil || r.Divergence.What != "options" || r.Divergence.Field != "options" {
		t.Fatalf("precondition: the repro is not an options divergence: %+v", r.Divergence)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = false
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	pol := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, pol, rc)
	// The precondition: the recorded prefix really was played, so the clean
	// result below is the engines agreeing past the old divergence, not a
	// script that silently stopped early.
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("precondition: script served %d of %d actions (%d skipped)", pol.next, len(r.Actions), pol.Skipped)
	}
	if res.Decisions < len(r.Actions) {
		t.Fatalf("precondition: replay stopped at %d decisions, before the recorded %d", res.Decisions, len(r.Actions))
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
}
