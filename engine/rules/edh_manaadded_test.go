package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// chooseColour records a real as-enters ChooseColor answer on o (the same
// events.Choose "color" event the real ETB ask's answer emits and
// foldChoose folds into state.Object.ChosenColor).
func chooseColour(t *testing.T, e *Engine, o state.ObjID, letter string) {
	t.Helper()
	e.emit(events.Event{Kind: events.Choose, Obj: o, Counter: "color", Text: letter})
	if got := e.G.Obj(o).ChosenColor; got != letter {
		t.Fatalf("precondition: ChosenColor = %q, want %q", got, letter)
	}
}

// TestEDHManaAddedAddsOneOfTheChosenColourFromALand: Caged Sun's printed
// text, "Whenever a land's ability causes you to add one or more mana of the
// chosen color, add an additional one mana of that color", resolving as a
// triggered mana ability (CR 605.1b): tapping a Forest for G while it has
// chosen green adds GG, with the bonus resolved immediately (no pending
// trigger, no stack object).
func TestEDHManaAddedAddsOneOfTheChosenColourFromALand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	sun := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Caged Sun"))
	chooseColour(t, e, sun, "G")
	forest := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Forest"))
	e.resolveManaAbility(0, forest, e.availableManaAbilities(0, forest)[0], false)
	if got := e.G.Players[0].Pool; got[4] != 2 || got.Total() != 2 {
		t.Fatalf("tapping a Forest under Caged Sun(chosen G) gave %+v, want 2 G", got)
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("the ManaAdded trigger left %d pending triggers, want none (CR 605.1b resolves immediately)", len(e.pendingTriggers))
	}
}

// TestEDHManaAddedRejectsWrongColourNonLandAndOpponent: the three gates of
// the printed text — the added mana must be of the CHOSEN colour, the
// producing source must be a land (ValidSource$ Land), and the add must be
// the carrier's own controller's (Player$ You).
func TestEDHManaAddedRejectsWrongColourNonLandAndOpponent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	// Wrong colour: a Swamp tapped while Caged Sun has chosen green.
	e := layerEngine(t)
	sun := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Caged Sun"))
	chooseColour(t, e, sun, "G")
	swamp := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Swamp"))
	e.resolveManaAbility(0, swamp, e.availableManaAbilities(0, swamp)[0], false)
	if p := e.G.Players[0].Pool; p[2] != 1 || p[4] != 0 || p.Total() != 1 {
		t.Fatalf("tapping a Swamp under Caged Sun(chosen G) gave %+v, want plain {B}", p)
	}
	// Non-land source (ValidSource$ Land): Sol Ring's artifact ability.
	e2 := layerEngine(t)
	sun2 := onBoardCard(t, e2, 0, mustCorpusCard(t, reg, "Caged Sun"))
	chooseColour(t, e2, sun2, "G")
	ring := onBoardCard(t, e2, 0, mustCorpusCard(t, reg, "Sol Ring"))
	e2.resolveManaAbility(0, ring, e2.availableManaAbilities(0, ring)[0], false)
	if p := e2.G.Players[0].Pool; p.Total() != 2 {
		t.Fatalf("Sol Ring under Caged Sun gave %+v, want plain {C}{C}", p)
	}
	// Opponent's land (Player$ You): seat 1 tapping while seat 0's Caged Sun
	// watches must not pay seat 1 anything extra.
	e3 := layerEngine(t)
	sun3 := onBoardCard(t, e3, 0, mustCorpusCard(t, reg, "Caged Sun"))
	chooseColour(t, e3, sun3, "B")
	forest3 := onBoardCard(t, e3, 1, mustCorpusCard(t, reg, "Forest"))
	e3.resolveManaAbility(1, forest3, e3.availableManaAbilities(1, forest3)[0], false)
	if p := e3.G.Players[1].Pool; p[4] != 1 || p.Total() != 1 {
		t.Fatalf("opponent's Forest under seat 0's Caged Sun gave %+v, want plain {G}", p)
	}
}

// TestEDHManaAddedPlannerPaysACastWithTwoForests: the payment planner must
// count the bonus mana so auto-pay and the legal actions agree — Juggernaut's
// {4} cost is funded by two Forests whose planned production is 2 G each
// under Caged Sun(chosen G), and the cast ends up paid on the stack with an
// empty pool and no pending trigger.
func TestEDHManaAddedPlannerPaysACastWithTwoForests(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	spell := mustCorpusCard(t, reg, "Juggernaut") // real {4} corpus cost
	e := handEngine(t, spell)
	sun := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Caged Sun"))
	chooseColour(t, e, sun, "G")
	f1 := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Forest"))
	f2 := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Forest"))
	spellID := e.G.Zone(state.ZHand, 0)[0]

	e.pending = nil
	e.beginCast(0, decision.Option{Kind: "cast", Obj: spellID})
	e.Advance()
	for d := e.Pending(); d != nil; d = e.Pending() {
		if o := e.G.Obj(spellID); o != nil && o.Zone == state.ZStack {
			// The cast is already on the stack and paid; any remaining ask
			// belongs to a later phase, not this payment.
			break
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "activate" && (o.Obj == f1 || o.Obj == f2) {
				idx = o.Index
				break
			}
		}
		if idx < 0 {
			t.Fatalf("payment ask %+v offers no Forest activation", d)
		}
		submitChoices(t, e, idx)
	}
	if o := e.G.Obj(spellID); o == nil || o.Zone != state.ZStack {
		t.Fatalf("Juggernaut zone = %+v, want paid cast on stack", o)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying {4} = %d, want empty", got)
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("Caged Sun left %d pending triggers, want its mana trigger resolved without the stack", len(e.pendingTriggers))
	}
}

// TestEDHManaAddedPlannerOffersTheCastOnlyWithTheFold pins the PLANNER half
// of the mechanism, the way TestTapManaFoldOfferedWhenExtraCompletesCost
// pins it for TapsForMana: two Forests alone (2 G) cannot fund Juggernaut's
// {4}, so the cast is not offered; with Caged Sun(chosen G) the planned
// production per Forest is 2 G, so it is offered with a real payment plan.
func TestEDHManaAddedPlannerOffersTheCastOnlyWithTheFold(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	spell := mustCorpusCard(t, reg, "Juggernaut")

	// Negative arm: no Caged Sun, 2 G cannot cover {4}.
	e := handEngine(t, spell)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Forest"))
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Forest"))
	if offered := manaAddedOffered(t, e, spell); offered {
		t.Fatal("two Forests alone offered the {4} cast")
	}

	// Positive arm: the folded bonus completes the cost.
	e2 := handEngine(t, spell)
	sun := onBoardCard(t, e2, 0, mustCorpusCard(t, reg, "Caged Sun"))
	chooseColour(t, e2, sun, "G")
	onBoardCard(t, e2, 0, mustCorpusCard(t, reg, "Forest"))
	onBoardCard(t, e2, 0, mustCorpusCard(t, reg, "Forest"))
	if !manaAddedOffered(t, e2, spell) {
		t.Fatal("Caged Sun(chosen G) + two Forests did not offer the {4} cast")
	}
	if e2.PlanCastPayment(0, paymentCast(e2.G.Zone(state.ZHand, 0)[0])).Plan == nil {
		t.Fatal("the offered cast has no payment plan")
	}
}

// manaAddedOffered reports whether PotentialActions offers casting the spell
// in hand, and asserts the spell is in hand first.
func manaAddedOffered(t *testing.T, e *Engine, spell *cards.Card) bool {
	t.Helper()
	hand := e.G.Zone(state.ZHand, 0)
	if len(hand) != 1 || e.G.Obj(hand[0]).Card != spell {
		t.Fatalf("precondition: hand = %v, want the spell alone", hand)
	}
	// The offer is read from the priority decision's memo: drive the round
	// first (the same driveToStep-then-priorityRound shape addMana uses).
	toMain1(t, e)
	e.priorityRound()
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "cast" && a.Obj == hand[0] {
			return true
		}
	}
	return false
}
