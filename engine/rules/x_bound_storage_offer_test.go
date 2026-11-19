package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestXBoundTargetOfferBoundsStorageMana pins CR 107.3i, 601.2b and 601.2h
// for an X-bounded activation: a storage land's "remove any number of storage
// counters: add that much {W}" is unbounded in PotentialMana (Amount$ X), but
// the activation can remove only the counters it has. One counter cannot pay
// an X of 2 even though the potential bound says it can, so the ability whose
// only target needs X=2 must not be offered; the unbounded bound used to
// offer it and the target ask then found no affordable announcement.
func TestXBoundTargetOfferBoundsStorageMana(t *testing.T) {
	t.Parallel()
	const store = "Name:Test Store\nManaCost:0\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ W | Amount$ X | SpellDescription$ Add {W} for each counter removed.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	const saga = "Name:Old Tale\nManaCost:2\nTypes:Enchantment Saga\nOracle:x\n"
	const summoner = "Name:Summoner\nTypes:Creature Human\nPT:1/1\n" +
		"A:AB$ ChangeZone | Cost$ X T | ValidTgts$ Saga.YouOwn+cmcEQX | TgtPrompt$ Pick a tale | Origin$ Graveyard | Destination$ Battlefield | SpellDescription$ Bring back a tale.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	e := handEngine(t)
	id := onBoardReady(t, e, 0, summoner)
	storeID := onBoardReady(t, e, 0, store)
	tale := e.G.AddObject(card(t, saga), 0)
	tale.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{tale.ID})
	e.emit(events.Event{Kind: events.CounterChange, Obj: storeID, Counter: "STORAGE", Amount: 1})
	if xBoundOffered(e, "ability", id) {
		t.Fatal("offered an X=2 activation with one storage counter and no other mana")
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: storeID, Counter: "STORAGE", Amount: 1})
	if !xBoundOffered(e, "ability", id) {
		t.Fatal("not offered when the two storage counters pay the X")
	}
}
