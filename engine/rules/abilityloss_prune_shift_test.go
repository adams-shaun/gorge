package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAbilityLossPruneKeepsALaterSourcesGrant pins CR 613.1f / 613.8a with
// the list order of dirac pair 93056: Witness Protection makes one
// Gnarlid Colony lose all abilities, so that Colony's "each creature you
// control with a +1/+1 counter has trample" stops applying -- but a second
// Colony that entered after a Garruk's Uprising keeps its own grant (CR
// 611.3a: each grant is its own instance, so the kicked second Colony has
// trample three times under two Uprisings). pruneLostSourceAbilities used to
// compact active()'s list while still deciding later sources, so the second
// Colony's loss check read the Uprising that had slid into Witness
// Protection's slot as a remover and dropped the Colony's own grant.
func TestAbilityLossPruneKeepsALaterSourcesGrant(t *testing.T) {
	t.Parallel()
	const colonyText = "ManaCost:1 G\nTypes:Creature Beast\nPT:2/2\n" +
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl+counters_GE1_P1P1 | AddKeyword$ Trample | Description$ x\nOracle:x\n"
	witness := corpusAlternativeCard(t, "Witness Protection")
	colonyA := card(t, "Name:Colony A\n"+colonyText)
	colonyB := card(t, "Name:Colony B\n"+colonyText)
	anthem := card(t, "Name:Test Uprising\nManaCost:2 G\nTypes:Enchantment\n"+
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Trample | Description$ x\nOracle:x\n")
	e, cfg, witnessID := corpusDeckEngine(t, []*cards.Card{witness}, []*cards.Card{colonyA, anthem, colonyB})
	ids := map[*cards.Card]state.ObjID{}
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.Zone == state.ZBattlefield {
			ids[o.Card] = o.ID
		}
	}
	aID, uID, bID := ids[colonyA], ids[anthem], ids[colonyB]
	if witnessID == 0 || aID == 0 || uID == 0 || bID == 0 {
		t.Fatalf("setup: witness=%d A=%d uprising=%d B=%d", witnessID, aID, uID, bID)
	}
	toMain1(t, e)
	addMana(t, e, 0, "U")
	submitChoices(t, e, castCardOption(t, e, witnessID).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("aura target decision: %+v", d)
	}
	ti := indexOfObjOption(d, aID)
	if ti < 0 {
		t.Fatalf("aura target ask does not offer Colony A: %+v", d.Options)
	}
	submitChoices(t, e, ti)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(witnessID).AttachedTo != aID {
		t.Fatalf("precondition: Witness Protection attached to %d, want Colony A %d", e.G.Obj(witnessID).AttachedTo, aID)
	}
	// The Uprising and then Colony B re-enter, so the timestamp order is
	// Colony A, Witness Protection, Uprising, Colony B -- pair 93056's.
	for _, id := range []state.ObjID{uID, bID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile, To: state.ZBattlefield})
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: aID, Counter: "P1P1", Amount: 2})
	e.emit(events.Event{Kind: events.CounterChange, Obj: bID, Counter: "P1P1", Amount: 2})
	trample := func(id state.ObjID) int {
		n := 0
		for _, kw := range e.Derived(id).Keywords {
			if kw == "Trample" {
				n++
			}
		}
		return n
	}
	// Colony B: the Uprising's grant and its own; Colony A's grant is gone.
	if got := trample(bID); got != 2 {
		t.Fatalf("Colony B trample instances = %d, want 2 (Uprising + its own); keywords %q", got, e.Derived(bID).Keywords)
	}
	// Colony A lost its abilities (its own grant is gone) but keeps the
	// later-timestamped grants of the Uprising and of Colony B, whose
	// "+1/+1 counter" grant reaches every creature its controller controls.
	if got := trample(aID); got != 2 {
		t.Fatalf("Colony A trample instances = %d, want 2 (Uprising + Colony B); keywords %q", got, e.Derived(aID).Keywords)
	}
	replayCheck(t, e, cfg)
}
