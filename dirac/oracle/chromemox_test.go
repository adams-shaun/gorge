package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Chrome Mox: when it enters you may exile a nonartifact, nonland card from
// your hand; {T}: add one mana of any of the exiled card's colors. gorge
// keeps the imprint on the source's Imprinted list, cleared when the Mox
// leaves the battlefield and pruned of a card that leaves exile; kgen lowers
// the face as the imprint-mox shape (dirac/kgen/lower_imprintmox.go) and the
// kernel reads the colours from the card's reverse exiled-with record at the
// Mox's current incarnation (dirac/mana_imprint.go). The pool imprints
// mono-coloured and two-colour cards (Boros Swiftblade: one colour choice
// among two), and Shatter and Boomerang make the Mox leave, so a recast Mox
// reads only its new imprint. The fixture pool is
// dirac/cardset/pools/fixtures/chromemox.

var chromeMoxDecks = []fixtureDecks{{
	{
		{Name: "Chrome Mox", Count: 8}, {Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Boros Swiftblade", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Chrome Mox", Count: 4}, {Name: "Shatter", Count: 4}, {Name: "Boomerang", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 10},
	},
}}

var chromeMoxWatch = map[string]bool{"Chrome Mox": true, "Shatter": true, "Boomerang": true, "Boros Swiftblade": true}

func TestChromeMoxLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.ExwStats
	dirac.DebugExwStats = &st
	defer func() { dirac.DebugExwStats = nil }()
	run := runFixtureLockstep(t, "chromemox", chromeMoxDecks, chromeMoxWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, imprints recorded %d", run.games, run.divs, run.stalls, run.casts, st.Recorded)
	if run.casts["Chrome Mox"] < 60 {
		t.Errorf("only %d Chrome Mox casts", run.casts["Chrome Mox"])
	}
	if st.Recorded < 30 {
		t.Errorf("only %d imprints recorded: the enters trigger rarely exiled a card", st.Recorded)
	}
}

// TestChromeMoxLockstepMutation: 1 drops the imprinted colours, so the Mox
// makes no mana (gorge taps it for the exiled card's colour); 2 appends a
// two-colour Mox's planner unit after the fixed sources instead of keeping
// it at its battlefield position (gorge's census replaces the stand-in it
// listed there). Each must diverge.
func TestChromeMoxLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []struct {
		mode uint8
		what string
	}{{1, "the Mox reflects no colour"}, {2, "a colour-choice Mox's census unit moves to the end"}} {
		t.Run(m.what, func(t *testing.T) {
			dirac.SetDebugImprintMutate(m.mode)
			defer dirac.SetDebugImprintMutate(0)
			run := runFixtureLockstep(t, "chromemox", chromeMoxDecks, chromeMoxWatch, 120, false, nil)
			run.requireDivergence(t, m.what)
		})
	}
}
