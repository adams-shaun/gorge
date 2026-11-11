package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Mox Diamond: "If Mox Diamond would enter, you may discard a land card
// instead. If you do, put Mox Diamond onto the battlefield. If you don't, put
// it into its owner's graveyard." gorge runs the optional discard (a yes/no
// election, then a pick among several lands) while the spell is still on the
// stack, then moves the card to the battlefield or the graveyard; no enters
// watcher sees a Mox that did not enter. kgen lowers the face as a
// ReplPayBeforeETB ability whose Prog is that discard, and the kernel enters
// the card or sends it to the graveyard when the program ends
// (dirac/repl_paybeforeetb.go). The pool pairs lands and spells in hand with
// an artifact-enters watcher (Reckless Fireweaver) and an opponent-discards
// watcher (Megrim), and Shatter and Boomerang make the Mox leave. The fixture
// pool is
// dirac/cardset/pools/fixtures/moxdiamond.

var moxDiamondDecks = []fixtureDecks{{
	{
		{Name: "Mox Diamond", Count: 8}, {Name: "Reckless Fireweaver", Count: 4}, {Name: "Megrim", Count: 2}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Mox Diamond", Count: 4}, {Name: "Megrim", Count: 4}, {Name: "Shatter", Count: 4}, {Name: "Boomerang", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Island", Count: 8},
	},
}}

var moxDiamondWatch = map[string]bool{"Mox Diamond": true, "Reckless Fireweaver": true, "Megrim": true, "Shatter": true}

func TestMoxDiamondLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.PayETBStats
	dirac.DebugPayETBStats = &st
	defer func() { dirac.DebugPayETBStats = nil }()
	run := runFixtureLockstep(t, "moxdiamond", moxDiamondDecks, moxDiamondWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, entered after a discard %d, sent to the graveyard %d",
		run.games, run.divs, run.stalls, run.casts, st.Entered, st.Graveyard)
	if run.casts["Mox Diamond"] < 60 {
		t.Errorf("only %d Mox Diamond casts", run.casts["Mox Diamond"])
	}
	if st.Entered < 30 || st.Graveyard < 30 {
		t.Errorf("the election was answered %d times for the discard and %d for the graveyard: a branch is rarely reached", st.Entered, st.Graveyard)
	}
}

// TestMoxDiamondLockstepMutation: a pay program that remembers nothing sends
// every Mox to the graveyard (gorge enters it after a discard), so it must
// diverge.
func TestMoxDiamondLockstepMutation(t *testing.T) {
	run := runFixtureLockstep(t, "moxdiamond", moxDiamondDecks, moxDiamondWatch, 120, false, func(tbl *kop.Table) {
		mutateCard(t, tbl, "Mox Diamond", func(a *kop.Ability, in *kop.Instr) {
			if a.Kind == kop.AbReplacement && in.Op == kop.OpDiscard {
				in.B = kop.Const(0) // no kop.DiscRemember
			}
		})
	})
	run.requireDivergence(t, "the pay-before-entering discard remembers nothing")
}
