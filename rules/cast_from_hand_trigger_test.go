// The Card.wasCastFromHandByYou trigger-side split (task castprov1), pinned
// end to end on real corpus carriers in both directions:
//
//   - Banish into Fable's `T:Mode$ SpellCast | ValidCard$
//     Card.Self+wasCastFromYourHandByYou` copy trigger: fires for a cast from
//     the hand, never for a card cheated into play.
//   - Epochrasite's etbCounter gate field `ValidCard$
//     Card.Self+!wasCastFromYourHandByYou`: the negated shape denies the
//     counters on a hand cast and grants them on a cheated entry.
//   - Jem Lightfoote's `SVar:X:Count$ThisTurnCast_Card.wasCastFromYourHand
//     ByYou` end-step gate (the count-path carriers): the "haven't cast a
//     spell from your hand this turn" trigger fires only when the provenance
//     count reads 0.
//
// The fixtures drive real casts (castMode/finishCast) and raw
// hand->battlefield entries (placeFromHand); no Forge .txt text is committed.

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func logHasStackCopyOf(e *Engine, from int, id state.ObjID) bool {
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.StackCopy && ev.Obj == id {
			return true
		}
	}
	return false
}

func logDrawsFor(e *Engine, from int, p state.PlayerID) int {
	n := 0
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.Draw && ev.Player == p {
			n++
		}
	}
	return n
}

func TestBanishIntoFableCastFromHandFiresTheCopyTrigger(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Banish into Fable"))
	bear := e.G.AddObject(card(t, "Name:Bear\nTypes:Creature\nPT:2/2\n"), 1)
	bear.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{bear.ID})
	addFillerPermanent(t, e, 0, "Name:Orn\nTypes:Artifact\nOracle:none") // the copy's ConditionPresent$ Artifact.YouCtrl
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MW], e.G.Players[0].Pool[state.MU] = 4, 1, 1
	n0 := len(e.L.Events)
	castMode(t, e, id, "")
	d := e.Pending()
	if d == nil || d.Kind != "target" && d.Kind != "target_effect" {
		// The target ask is the cast's target step; identify it by its
		// options rather than assuming the kind string.
		if d == nil || len(d.Options) == 0 {
			t.Fatalf("no target ask for the banish cast: %+v", d)
		}
	}
	idx := -1
	for i, o := range d.Options {
		if o.Obj == bear.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("banish target ask missing the bear: %+v", d)
	}
	submitChoices(t, e, idx)
	finishCast(t, e, id)
	passUntilStackEmpty(t, e, 60)
	if !logHasStackCopyOf(e, n0, id) {
		t.Fatal("cast-from-hand Banish into Fable fired no copy trigger (no StackCopy of the spell in the log)")
	}
	if o := e.G.Obj(bear.ID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("bear after the banish resolution = %v, want the owner's hand", o)
	}
}

func TestBanishIntoFableCheatedIntoPlayFiresNothing(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Banish into Fable"))
	bear := e.G.AddObject(card(t, "Name:Bear\nTypes:Creature\nPT:2/2\n"), 1)
	bear.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{bear.ID})
	id := e.G.Zone(state.ZHand, 0)[0]
	n0 := len(e.L.Events)
	placeFromHand(t, e, id)
	if logHasStackCopyOf(e, n0, id) {
		t.Fatal("cheated-into-play Banish into Fable fired the cast-from-hand copy trigger")
	}
	if o := e.G.Obj(bear.ID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("bear moved with no resolution: %v", o)
	}
}

func TestEpochrasiteCastFromHandEntersWithoutCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Epochrasite"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC] = 2
	castMode(t, e, id, "")
	finishCast(t, e, id)
	if got := e.G.Obj(id).Counter("P1P1"); got != 0 {
		t.Fatalf("cast-from-hand Epochrasite entered with %d P1P1, want 0 (the !wasCastFromYourHandByYou gate must deny)", got)
	}
}

func TestEpochrasiteCheatedIntoPlayEntersWithThree(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Epochrasite"))
	id := e.G.Zone(state.ZHand, 0)[0]
	placeFromHand(t, e, id)
	if got := e.G.Obj(id).Counter("P1P1"); got != 3 {
		t.Fatalf("cheated Epochrasite entered with %d P1P1, want 3 (the oracle's 'if you didn't cast it from your hand')", got)
	}
}

// TestJemLightfooteCastFromHandEndStepDrawsNothing pins the count-path
// carriers: Jem's end-step "if you haven't cast a spell from your hand this
// turn, draw a card" trigger gates on
// Count$ThisTurnCast_Card.wasCastFromYourHandByYou, which the resolving cast
// itself satisfies -- so the trigger never fires on the turn you cast her.
func TestJemLightfooteCastFromHandEndStepDrawsNothing(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Jem Lightfoote, Sky Explorer"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MW], e.G.Players[0].Pool[state.MU] = 2, 1, 1
	castMode(t, e, id, "")
	finishCast(t, e, id)
	n0 := len(e.L.Events)
	driveToStepAll(t, e, 1, 0, state.StepEnd)
	drainQueuedTriggers(t, e)
	if draws := logDrawsFor(e, n0, 0); draws != 0 {
		t.Fatalf("Jem Lightfoote's end step drew %d cards on a turn she was cast from hand, want 0", draws)
	}
}

func TestJemLightfooteCheatedIntoPlayEndStepDraws(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Jem Lightfoote, Sky Explorer"))
	id := e.G.Zone(state.ZHand, 0)[0]
	placeFromHand(t, e, id)
	n0 := len(e.L.Events)
	driveToStepAll(t, e, 1, 0, state.StepEnd)
	drainQueuedTriggers(t, e)
	if draws := logDrawsFor(e, n0, 0); draws != 1 {
		t.Fatalf("cheated Jem Lightfoote's end step drew %d cards, want 1 (the provenance count reads 0)", draws)
	}
}
