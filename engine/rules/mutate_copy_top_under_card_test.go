package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMutateCopyOnTopKeepsTheDemotedCard pins that a top merge's under-card
// takes nothing from the resolving spell's object. A copy of a mutating spell
// (Double Major) mutates too; if the demoted real card carried the copy mark of
// that spell, CR 704.5e would cease it when the pile dies and the player's card
// would be lost instead of going to the graveyard (CR 730.3).
func TestMutateCopyOnTopKeepsTheDemotedCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	phoenix := mustCorpusCard(t, reg, "Everquill Phoenix")
	major := mustCorpusCard(t, reg, "Double Major")
	e, cfg := tokenReplGame(t, 402, phoenix, major, card(t, mutateBearSrc))
	phoenixID := moveSeededCard(t, e, 0, phoenix, state.ZHand)
	majorID := moveSeededCard(t, e, 0, major, state.ZHand)
	// A real deck card, not a token: only a real card can be lost.
	bear := moveSeeded(t, e, 0, mutateBearSrc, state.ZBattlefield)
	addMana(t, e, 0, "RRRR")

	mutateCastOntoUndrained(t, e, mutatedCastOption(t, e, phoenixID), bear, true)

	// Double Major copies the mutating spell while it is on the stack. Its
	// mana is added now so the mutate cost cannot have spent it; the
	// priority window is re-asked without leaving the stack's step.
	for _, c := range []string{"G", "U"} {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: c, Amount: 1})
	}
	e.pending = nil
	e.priorityRound()
	submitChoices(t, e, castOptionFor(t, e, majorID).Index)
	d := e.Pending()
	if d == nil || indexOfObjOption(d, phoenixID) < 0 {
		t.Fatalf("precondition: Double Major does not target the mutating spell: %+v", d)
	}
	submitChoices(t, e, indexOfObjOption(d, phoenixID))
	for i := 0; i < 10 && e.G.Obj(majorID).Zone == state.ZStack; i++ {
		passPriorityOnce(t, e)
	}
	if len(e.G.Stack) != 2 {
		t.Fatalf("precondition: stack = %v, want the spell and its copy", e.G.Stack)
	}
	dup := e.G.Stack[len(e.G.Stack)-1]
	if o := e.G.Obj(dup); dup == phoenixID || !o.IsCopy || o.CastFlags&state.FlagMutatedTop == 0 {
		t.Fatalf("precondition: top of stack %d is not a mutating-on-top copy: %+v", dup, o)
	}
	for i := 0; i < 10 && e.G.Obj(dup).Zone == state.ZStack; i++ {
		passPriorityOnce(t, e)
	}

	pile := e.G.Obj(bear)
	if pile.Zone != state.ZBattlefield || len(pile.MergedCards) != 1 {
		t.Fatalf("precondition: the copy did not merge on top: %+v", pile)
	}
	under := pile.MergedCards[0].Obj
	if o := e.G.Obj(under); o.IsCopy || o.IsToken || o.Owner != 0 {
		t.Errorf("demoted real card is marked copy=%v token=%v owner=%d", o.IsCopy, o.IsToken, o.Owner)
	}

	// The pile dies; state-based actions run before the next priority.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.checkStateBased()

	got := e.G.Obj(under)
	if got.Zone != state.ZGraveyard || got.Face() == nil || got.Face().Name != "Mutate Bear" {
		t.Fatalf("demoted card: zone=%v, want Mutate Bear in the graveyard", got.Zone)
	}
	replayCheck(t, e, cfg)
}
