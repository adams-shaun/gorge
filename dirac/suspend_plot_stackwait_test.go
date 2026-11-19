package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 702.62a with CR 117.1a: the suspend special action is taken "any time you
// could begin to cast this card", so it follows the card's own cast timing --
// an instant's (or flashed card's) suspension is offered with the stack
// non-empty, like the instant cast beside it; a sorcery-timed card's
// suspension waits for the stack to empty
// (mbc_suspend_special_action_waits_for_trigger_stack_lockstep_test.go).
// Plot (CR 702.170a) is its own sorcery-speed special action whatever the
// face's own type is: even an instant's plot action waits (gorge's plot gate,
// legal_walk_hand.go).
func TestSpecialActionStackWaitFollowsTheCard(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	instSusp := CardID(len(tbl.Cards))
	creaSusp := instSusp + 1
	instPlot := instSusp + 2
	sorcPlot := instSusp + 3
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Instant Suspend", Types: kop.TInstant, Suspend: 3, SuspendCost: kop.ManaCost{Generic: 1}},
		kop.CardDef{Name: "Test Creature Suspend", Types: kop.TCreature, Suspend: 3, SuspendCost: kop.ManaCost{Generic: 1}},
		kop.CardDef{Name: "Test Instant Plot", Types: kop.TInstant, HasPlot: true, PlotCost: kop.ManaCost{Generic: 2}},
		kop.CardDef{Name: "Test Sorcery Plot", Types: kop.TSorcery, HasPlot: true, PlotCost: kop.ManaCost{Generic: 2}},
	)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	suspendOf := func(c CardID) Action {
		o := sc.hand(c, 0)
		return MakeAction(AkPlot, TCode(o), 0, plotModeSuspend)
	}
	plotOf := func(c CardID) Action {
		o := sc.hand(c, 0)
		return MakeAction(AkPlot, TCode(o), 0, 0)
	}
	castOf := func(c CardID) Action {
		o := sc.hand(c, 0)
		return MakeAction(AkCast, TCode(o), 0, 0)
	}
	// The precondition the suspend cases lean on: the same action differs
	// only by its card, and the plain casts already differ (the creature's
	// cast is sorcery-speed, the instant's is not).
	if !sc.s.castNeedsEmptyStack(sc.k, castOf(creaSusp)) {
		t.Fatal("the creature's plain cast should wait for the stack (sorcery timing)")
	}
	if sc.s.castNeedsEmptyStack(sc.k, castOf(instSusp)) {
		t.Fatal("the instant's plain cast should not wait for the stack")
	}
	if sc.s.specialNeedsEmptyStack(sc.k, suspendOf(creaSusp)) != sc.s.castNeedsEmptyStack(sc.k, castOf(creaSusp)) {
		t.Error("the creature's suspension should time as its cast: wait for the stack (CR 702.62a)")
	}
	if sc.s.specialNeedsEmptyStack(sc.k, suspendOf(instSusp)) != sc.s.castNeedsEmptyStack(sc.k, castOf(instSusp)) {
		t.Error("the instant's suspension should time as its cast: not wait for the stack (CR 702.62a, Suspended Sentence)")
	}
	// The plot witness: the plot action is sorcery-speed on its own (CR
	// 702.170a), even for an instant face, so both plots wait.
	if !sc.s.specialNeedsEmptyStack(sc.k, plotOf(sorcPlot)) {
		t.Error("the sorcery's plot action should wait for the stack (CR 702.170a)")
	}
	if !sc.s.specialNeedsEmptyStack(sc.k, plotOf(instPlot)) {
		t.Error("even the instant's plot action should wait for the stack (CR 702.170a)")
	}
}
