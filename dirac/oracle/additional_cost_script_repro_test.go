package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestStdEclAdditionalCostScriptReprosLockstep pins the pair-92 divergence at
// decision 836 of the std-ecl sweep: Champions of the Perfect ({3}{G}, "as an
// additional cost to cast this spell, behold an Elf and exile it") was offered
// by dirac but not by gorge once the fourth mana came from Springleaf Drum, a
// source the planner's census cannot price. A plain cast from the hand does
// not follow the census/prefix search in gorge (its planner returns
// shape:additional_cost for the behold-exile cost), so dirac's lowering must
// not offer it either: both engines now list only the Bristlebane Outrider
// cast at that priority (CR 601.2b, 601.2g-h).
//
// The fixture is the recorded 836-decision prefix and the full-surface bundle
// it diverged in. The replay tail is FirstPolicy for 200 further decisions, so
// a fix that merely moves the disagreement later still fails.
func TestStdEclAdditionalCostScriptReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-ecl")
	r, err := ReadRepro(filepath.Join("testdata", "repro", "additional_cost_hand_cast_stays_unoffered.repro.json"))
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
	// result below is the engines agreeing at decision 836, not a script that
	// silently stopped early.
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("precondition: script served %d of %d actions (%d skipped)", pol.next, len(r.Actions), pol.Skipped)
	}
	if res.Decisions < 836 {
		t.Fatalf("precondition: replay stopped at %d decisions, before the divergence at 836", res.Decisions)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
}
