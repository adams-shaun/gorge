package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Mox Amber: "{T}: Add one mana of any color among legendary creatures and
// planeswalkers you control." gorge's planner census lists it as one
// reflected unit over the colours its resolution's object walk names now
// (rules.paymentPlanReflectedAlt), no ability at all with none; kgen lowers
// it as a lone AfManaReflect with a ManaAmong filter and the kernel reads
// the filter's colours into the imprint mox's per-colour views
// (dirac/mana_imprint.go prepManaImprint, mana_reflected.go reflColours).
// The pool has a one-colour legend (Isamaru), a two-colour one (Tymna: a
// colour choice), a planeswalker (Jace Beleren), and Lightning Bolt to kill
// a legend, so the colours move mid-game. The fixture pool is
// dirac/cardset/pools/fixtures/moxamber.

var moxAmberDecks = []fixtureDecks{{
	{
		{Name: "Mox Amber", Count: 8}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Bartolomé del Presidio", Count: 4},
		{Name: "Jace Beleren", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Plains", Count: 6},
		{Name: "Swamp", Count: 5}, {Name: "Island", Count: 5},
	},
	{
		{Name: "Mox Amber", Count: 4}, {Name: "Lightning Bolt", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Bartolomé del Presidio", Count: 2}, {Name: "Mountain", Count: 10},
		{Name: "Plains", Count: 3}, {Name: "Swamp", Count: 3},
	},
}}

var moxAmberWatch = map[string]bool{
	"Mox Amber": true, "Isamaru, Hound of Konda": true, "Bartolomé del Presidio": true, "Jace Beleren": true, "Lightning Bolt": true,
}

func TestMoxAmberLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "moxamber", moxAmberDecks, moxAmberWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Mox Amber"] < 60 {
		t.Errorf("only %d Mox Amber casts", run.casts["Mox Amber"])
	}
	if run.casts["Isamaru, Hound of Konda"]+run.casts["Bartolomé del Presidio"]+run.casts["Jace Beleren"] < 60 {
		t.Errorf("only %d legend casts", run.casts["Isamaru, Hound of Konda"]+run.casts["Bartolomé del Presidio"]+run.casts["Jace Beleren"])
	}
}

// TestMoxAmberLockstepMutation: the first switch drops the reflected colours,
// so the Mox makes no mana (gorge taps it for a legend's colour); the second
// moves a colour-choice Mox's planner unit to the end of the census instead
// of its battlefield position. Each must diverge.
func TestMoxAmberLockstepMutation(t *testing.T) {
	for _, m := range []struct {
		what string
		set  func(bool)
	}{
		{"the Mox reflects no colour", func(on bool) { dirac.SetDebugReflMutate(map[bool]uint8{true: 1}[on]) }},
		{"a colour-choice Mox's census unit moves to the end", func(on bool) { dirac.SetDebugImprintMutate(map[bool]uint8{true: 2}[on]) }},
	} {
		t.Run(m.what, func(t *testing.T) {
			m.set(true)
			defer m.set(false)
			run := runFixtureLockstep(t, "moxamber", moxAmberDecks, moxAmberWatch, 120, false, nil)
			run.requireDivergence(t, m.what)
		})
	}
}
