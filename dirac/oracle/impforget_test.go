package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// The classic Imprint artifacts: Mirror Golem's forget line fires when an
// imprinted card leaves exile, Duplicant's TrigExile replaces the imprinted
// card (Unimprint$/ImprintLast$ clear before the move), and Mirror of Life
// Trapping's TrigReturn returns the exiled-with permanents that are NOT
// imprinted (!IsImprinted). gorge keeps each list on the source
// (Object.Imprinted); the kernel keeps it in the per-source memory
// (CardDef.ImpPersist, dirac/zone_imprint.go) and seeds each resolution of
// the source from it. Naturalize makes a source leave.
// The fixture pool is dirac/cardset/pools/fixtures/impforget.

var impForgetDecks = []fixtureDecks{{
	{
		{Name: "Duplicant", Count: 4}, {Name: "Mirror Golem", Count: 3},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Naturalize", Count: 3}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Mirror of Life Trapping", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Hill Giant", Count: 6}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Murder", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
}}

var impForgetWatch = map[string]bool{
	"Duplicant": true, "Mirror Golem": true, "Mirror of Life Trapping": true, "Naturalize": true,
}

func TestImpForgetLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.SrcMemStats
	dirac.DebugSrcMemStats = &st
	defer func() { dirac.DebugSrcMemStats = nil }()
	run := runFixtureLockstep(t, "impforget", impForgetDecks, impForgetWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, imprints recorded %d (seeded %d, matched %d, cleared %d, forgot %d)",
		run.games, run.divs, run.stalls, run.casts, st.Recorded, st.Seeded, st.Matched, st.Cleared, st.Forgot)
	if run.divs != 0 || run.stalls != 0 {
		t.Errorf("%d divergences, %d stalls; want none", run.divs, run.stalls)
	}
	for _, name := range []string{"Duplicant", "Mirror Golem", "Mirror of Life Trapping"} {
		if run.casts[name] < 15 {
			t.Errorf("only %d %s casts", run.casts[name], name)
		}
	}
	if st.Cleared < 10 {
		t.Errorf("only %d store records cleared: the clears rarely resolved", st.Cleared)
	}
}

// TestImpForgetMutation: with W7NotImprinted answered as W7Imprinted, Mirror
// of Life Trapping's TrigReturn pick (Permanent.ExiledWithSource+!IsImprinted)
// returns the creature it just exiled instead of leaving it set aside.
func TestImpForgetMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugW7NotImprintedFlip = true
	defer func() { dirac.DebugW7NotImprintedFlip = false }()
	run := runFixtureLockstep(t, "impforget", impForgetDecks, impForgetWatch, 120, false, nil)
	run.requireDivergence(t, "Mirror of Life Trapping returns the creature it just exiled")
}
