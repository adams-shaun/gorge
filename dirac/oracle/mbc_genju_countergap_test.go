package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestMBCGenjuCounterGapReprosLockstep pins the pair-62 divergence of the
// dr-mbc-bbba sweep at decision 773: Genju of the Fields' {2} activation was
// offered by gorge and not by the kernel when the only mana left floats from
// a depletion land's rider ability beside a tap-another-permanent census gap
// (Birchlore Rangers' "Tap two untapped Elves you control"). gorge's prefix
// search is one search over every census gap (rules scriptSearch.visit), so
// it activates both the Birchlore and the Sandstone Needle and then pays the
// {2}; the kernel's tap-another search (mana_tapanother.go) priced only its
// own gaps and dropped the play. The search now covers the Taps-deferred
// rider sources beside the tap-another ones (CR 601.2g-h, 605.1a-b).
//
// The fixture is the recorded 773-decision prefix and the full-surface bundle
// it diverged in. The replay tail is FirstPolicy for 200 further decisions, so
// a fix that merely moves the disagreement later still fails.
func TestMBCGenjuCounterGapReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "dr-mbc-bbba")
	r, err := ReadRepro(filepath.Join("testdata", "repro", "tapx_search_skips_deferred_rider_gap.repro.json"))
	if err != nil {
		t.Fatal(err)
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
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	pol := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, pol, rc)
	// The precondition: the recorded prefix really was played, so the clean
	// result below is the engines agreeing through the divergence and past it,
	// not a script that silently stopped early.
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("precondition: script served %d of %d actions (%d skipped)", pol.next, len(r.Actions), pol.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= len(r.Actions) {
		t.Fatalf("precondition: replay stopped at %d decisions, at or before the divergence at %d", res.Decisions, len(r.Actions))
	}
}
