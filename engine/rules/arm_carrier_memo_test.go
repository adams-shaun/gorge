package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestArmCarrierMemoTracksFaceRewrites pins the incremental
// PaymentPlanInterferenceCarriers memo's freshness: the list may only reuse
// its previous build while no object was created and no event that rewrites a
// card or a mutated pile ran. Mutate adds an under-card, a battlefield
// MoveZone clears the pile, and the grown object arena extends the list with
// the new objects.
func TestArmCarrierMemoTracksFaceRewrites(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9422, ppFiller)
	carrierSrc := "Name:Arm Carrier Source\nManaCost:1\nTypes:Creature\nPT:1/1\n" +
		"T:Mode$ TapsForMana | Execute$ TrigMana | TriggerDescription:x\n" +
		"Oracle:x\n"
	plainSrc := "Name:Arm Carrier Plain\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n"

	enter := func(src string) state.ObjID {
		t.Helper()
		id := handCard(e, card(t, src), 0)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
		return id
	}
	// The victim enters first so its id orders before the carrier's.
	victim := enter(plainSrc)
	mutant := enter(carrierSrc)
	plain := enter(plainSrc)

	// The memo's first build: the carrier is listed, the plain objects are
	// not (the preconditions the later assertions depend on).
	got := pay.PaymentPlanInterferenceCarriers(asPayer(e))
	if !slices.Contains(got, mutant) {
		t.Fatalf("first build misses carrier %d: %v", mutant, got)
	}
	for _, id := range []state.ObjID{victim, plain} {
		if slices.Contains(got, id) {
			t.Fatalf("first build lists plain object %d: %v", id, got)
		}
	}

	// Mutate puts the carrier's card under the victim: the victim now carries
	// the interference text, and the memo must not serve the pre-mutation
	// list.
	e.emit(events.Event{Kind: events.Mutate, Obj: victim, IDs: []state.ObjID{mutant}, Text: "under", Amount: 1})
	got = pay.PaymentPlanInterferenceCarriers(asPayer(e))
	if !slices.Contains(got, victim) {
		t.Fatalf("after Mutate, carrier list %v misses the mutated pile %d", got, victim)
	}
	if o := e.G.Obj(victim); o == nil || len(o.MergedCards) == 0 {
		t.Fatalf("precondition: Mutate left no under-card on %d", victim)
	}

	// A battlefield MoveZone clears the pile: the victim stops carrying the
	// text and the memo must not serve the mutated list.
	e.emit(events.Event{Kind: events.MoveZone, Obj: victim, From: state.ZBattlefield, To: state.ZGraveyard})
	got = pay.PaymentPlanInterferenceCarriers(asPayer(e))
	if slices.Contains(got, victim) {
		t.Fatalf("after leaving the battlefield, carrier list %v still lists %d", got, victim)
	}

	// A new object (the arena grows with no event) is tested and appended,
	// not missed by a stale full-list memo.
	grown := e.G.AddObject(card(t, carrierSrc), 0)
	got = pay.PaymentPlanInterferenceCarriers(asPayer(e))
	if !slices.Contains(got, grown.ID) {
		t.Fatalf("grown-arena carrier list %v misses new carrier %d", got, grown.ID)
	}
}
