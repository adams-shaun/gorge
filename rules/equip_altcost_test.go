// Task cli-20260923T060000Z-equip-altcost (the eqcm1 row's AlternateCost$
// sub-shape): the K:Equip expansion dropped an equip's AlternateCost$ rider,
// so an equip with an alternative cost offered only the printed one. These
// leaves pin the corrected behaviour on the real corpus cards the row names:
// Transmogrant's Crown (`K:Equip:2:::AlternateCost$ B`, an alternative MANA
// cost), Bloodthorn Flail (`AlternateCost$ Discard<1/Card>`, a non-mana cost
// paid through the ordinary payment stages) and Gavel of the Righteous
// (`AlternateCost$ RemoveAnyCounter<...>`), each through the ordinary
// offer/charge path, never copied script text.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// findAbilityCostOption returns the "ability" option for id at flat ability
// index idx whose AltCostIndex equals want (0 = the printed Cost$, 1 = the
// ability's own AlternateCost$ rider). It is findAbilityOption with the cost
// selector added, since the two options share Obj/Ability.
func findAbilityCostOption(e *Engine, id state.ObjID, idx, want int) (decision.Option, bool) {
	d := e.Pending()
	if d == nil {
		return decision.Option{}, false
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == id && o.Ability == idx && o.AltCostIndex == want {
			return o, true
		}
	}
	return decision.Option{}, false
}

// equipAltSA resolves the equipment's ability idx and asserts its rider
// structure (a precondition, so a changed corpus line fails loudly rather
// than passing silently).
func equipAltSA(t *testing.T, e *Engine, id state.ObjID, idx int, printedGeneric int32, wantAlt bool) *cards.SA {
	t.Helper()
	o := e.G.Obj(id)
	pa, ok := o.PileAbilityAt(idx)
	if !ok {
		t.Fatalf("equipment %d has no ability at index %d", id, idx)
	}
	if got := e.parseCost(pa.SA.Params["Cost"]).Generic; got != printedGeneric {
		t.Fatalf("printed equip generic cost = %d, want %d (the corpus line changed)", got, printedGeneric)
	}
	_, okAlt := e.abilityAlternateCost(pa.SA)
	if okAlt != wantAlt {
		t.Fatalf("abilityAlternateCost ok = %v, want %v (riders = %+v)", okAlt, wantAlt, pa.SA.Params)
	}
	return pa.SA
}

// addHandCard moves seat p's top library card into its hand with a logged
// MoveZone (the same shape rules/attach_optional_test.go's findAndMoveToHand
// emits), returning the card id and its name.
func addHandCard(t *testing.T, e *Engine, p state.PlayerID) state.ObjID {
	t.Helper()
	lib := e.G.Zone(state.ZLibrary, p)
	if len(lib) == 0 {
		t.Fatalf("seat %d's library is empty", p)
	}
	id := lib[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
	return id
}

// TestEquipAlternateCostManaOfferedAndChargedOnTransmograntsCrown drives the
// real corpus Transmogrant's Crown ("Equip {2} ... pay {B} instead"): with
// only {B} floating the printed {2} is unpayable, so the alternate option must
// be offered, and it must charge exactly the {B} and attach.
func TestEquipAlternateCostManaOfferedAndChargedOnTransmograntsCrown(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := linkBoard(t, reg, []string{"Transmogrant's Crown", "Grizzly Bears"}, nil)
	crown := findOnBoard(t, e, 0, "Transmogrant's Crown")
	bear := findOnBoard(t, e, 0, "Grizzly Bears")
	e.priorityRound()
	sa := equipAltSA(t, e, crown, 0, 2, true)
	// Precondition: the alternate cost really is a ONE-BLACK-MANA cost that
	// differs from the printed {2}, so the two options are distinguishable.
	alt, _ := e.abilityAlternateCost(sa)
	if alt.Generic != 0 || alt.Colored[state.MB] != 1 {
		t.Fatalf("alternate cost = %+v, want exactly {B}", alt)
	}
	// Float exactly {B}: the printed {2} cannot be paid, the alternate can.
	addMana(t, e, 0, "B")
	if got := e.G.Players[0].Pool.Total(); got != 1 {
		t.Fatalf("pool = %d, want 1 (only the alternate {B} is payable)", got)
	}
	if _, ok := findAbilityCostOption(e, crown, 0, 0); ok {
		t.Fatal("printed-cost equip option offered from a {B}-only pool, but it costs {2}")
	}
	opt, ok := findAbilityCostOption(e, crown, 0, 1)
	if !ok {
		t.Fatal("Transmogrant's Crown's alternate-cost equip option not offered with {B} floating")
	}
	submitChoices(t, e, opt.Index)
	targetObject(t, e, bear)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(crown).AttachedTo != bear {
		t.Fatalf("Crown attached to %d, want creature %d", e.G.Obj(crown).AttachedTo, bear)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the alternate-cost equip = %d, want 0 (exactly {B} charged)", got)
	}
	replayCheck(t, e, cfg)
}

// TestEquipAlternateCostWithheldWhenNeitherPayable pins the fail-closed offer
// gate: Bloodthorn Flail's printed {3} is unpayable from an empty pool and its
// discard alternate is unpayable from an empty hand, so NEITHER option may be
// offered.
func TestEquipAlternateCostWithheldWhenNeitherPayable(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _ := linkBoard(t, reg, []string{"Bloodthorn Flail", "Grizzly Bears"}, nil)
	flail := findOnBoard(t, e, 0, "Bloodthorn Flail")
	equipAltSA(t, e, flail, 0, 3, true)
	// Empty the hand and only THEN re-offer priority, so the offer walk sees
	// the empty hand. (linkBoard leaves a stale pending decision; mutating
	// state after priorityRound would leave that stale decision in place.)
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	e.priorityRound()
	if got := len(e.G.Zone(state.ZHand, 0)); got != 0 {
		t.Fatalf("hand = %d cards, want 0 (the discard cost must be unpayable)", got)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool = %d, want 0 (the printed {3} must be unpayable)", got)
	}
	if _, ok := findAbilityCostOption(e, flail, 0, 0); ok {
		t.Fatal("printed-cost equip offered with no mana")
	}
	if opt, ok := findAbilityCostOption(e, flail, 0, 1); ok {
		t.Fatalf("alternate-cost equip offered with an empty hand: %+v", opt)
	}
}

// TestEquipAlternateCostDiscardOfferedAndChargedOnBloodthornFlail drives the
// real corpus Bloodthorn Flail ("Equip {3} ... discard a card instead"): with
// no mana but a card in hand, the discard-priced option must be offered,
// activate, discard the card, spend no mana, and attach.
func TestEquipAlternateCostDiscardOfferedAndChargedOnBloodthornFlail(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := linkBoard(t, reg, []string{"Bloodthorn Flail", "Grizzly Bears"}, nil)
	flail := findOnBoard(t, e, 0, "Bloodthorn Flail")
	bear := findOnBoard(t, e, 0, "Grizzly Bears")
	sa := equipAltSA(t, e, flail, 0, 3, true)
	// Precondition: the alternate cost is a one-card DISCARD, not mana.
	alt, _ := e.abilityAlternateCost(sa)
	if alt.Generic != 0 || len(alt.Discard) != 1 {
		t.Fatalf("alternate cost = %+v, want exactly one Discard part", alt)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool = %d, want 0", got)
	}
	if len(e.G.Zone(state.ZHand, 0)) == 0 {
		addHandCard(t, e, 0)
	}
	before := len(e.G.Zone(state.ZHand, 0))
	if before == 0 {
		t.Fatal("hand is empty; the discard cost is unpayable and the offer below proves nothing")
	}
	e.priorityRound()
	if _, ok := findAbilityCostOption(e, flail, 0, 0); ok {
		t.Fatal("printed-cost equip offered with no mana")
	}
	opt, ok := findAbilityCostOption(e, flail, 0, 1)
	if !ok {
		t.Fatal("Bloodthorn Flail's alternate-cost equip option not offered with a card in hand")
	}
	submitChoices(t, e, opt.Index)
	// The discard cost stage asks which card to discard.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want a discard KChoose after choosing the alternate cost, got %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	targetObject(t, e, bear)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(flail).AttachedTo != bear {
		t.Fatalf("Flail attached to %d, want creature %d", e.G.Obj(flail).AttachedTo, bear)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != before-1 {
		t.Fatalf("hand after the alternate-cost equip = %d, want %d (one card discarded)", got, before-1)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the discard-priced equip = %d, want 0 (no mana spent)", got)
	}
	replayCheck(t, e, cfg)
}
