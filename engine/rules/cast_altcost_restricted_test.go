package rules

// Follow-up probe for the may-play twobrid + delve offer (CR 107.4e, 702.66a):
// a twobrid pip's generic face is only payable if the mana paying it may pay a
// generic cost. A floating CantPayGenericCosts batch (Jegantha, the Wellspring;
// The Rebellious Intelligence) is admitted to a still-unresolved all-twobrid
// cost because that cost has no generic part yet; once a face announces its
// {2}, the batch is no longer spendable on it. castable must therefore price
// each announcement leaf against its own restriction-adjusted pool, not the
// unresolved cost's, or it offers a cast whose only payment is the batch the
// announcement ask will then refuse.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// twobridRestrictedEngine is twobridDelveEngine's shape with two artifact
// mana sources whose batch is CantPayGenericCosts-only, one producing W and
// one R -- colours Gurmag Nightwatch's {2/B}{2/G}{2/U} cannot spend on its
// coloured faces. The graveyard holds gy cards, so the delve credit can cover
// only that many of the announced generic faces.
func twobridRestrictedEngine(t *testing.T, reg *cards.Registry, gy int) (*Engine, state.ObjID) {
	t.Helper()
	gurmag := twobridGurmag(t)
	teval := twobridTeval(t)
	bear := card(t, "Name:Bear Probe\nManaCost:0\nTypes:Creature Bear\nPT:1/1\nOracle:x")
	whiteRock := card(t, "Name:Restricted White Rock\nManaCost:0\nTypes:Artifact\n"+
		"A:AB$ Mana | Cost$ T | Produced$ W | RestrictValid$ CantPayGenericCosts | SpellDescription$ Add {W}.\nOracle:x")
	redRock := card(t, "Name:Restricted Red Rock\nManaCost:0\nTypes:Artifact\n"+
		"A:AB$ Mana | Cost$ T | Produced$ R | RestrictValid$ CantPayGenericCosts | SpellDescription$ Add {R}.\nOracle:x")
	deck0 := []*cards.Card{gurmag, teval, whiteRock, redRock}
	for len(deck0) < 40 {
		deck0 = append(deck0, bear)
	}
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = lookup(t, reg, "Mountain")
	}
	e := New(Config{Seed: 42, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{deck0, deck1}})
	e.Advance()
	toMain1(t, e)
	// Keep the board exactly the fixture's: the spell in hand, Teval and the
	// two restricted rocks in play, gy bears in the graveyard.
	for _, id := range append([]state.ObjID{}, e.G.Zone(state.ZHand, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	gid := pullByName(t, e, 0, twobridGurmagName)
	e.emit(events.Event{Kind: events.MoveZone, Obj: gid, From: e.G.Obj(gid).Zone, To: state.ZHand})
	var whiteID, redID state.ObjID
	for _, name := range []string{"Teval Arbiter Probe", "Restricted White Rock", "Restricted Red Rock"} {
		id := pullByName(t, e, 0, name)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
		switch name {
		case "Restricted White Rock":
			whiteID = id
		case "Restricted Red Rock":
			redID = id
		}
	}
	for i := 0; i < gy; i++ {
		id := pullByName(t, e, 0, "Bear Probe")
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZGraveyard})
	}
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != gy {
		t.Fatalf("precondition: graveyard holds %d cards, want %d", got, gy)
	}
	if whiteID == 0 || redID == 0 {
		t.Fatalf("precondition: restricted rocks not found (white=%d red=%d)", whiteID, redID)
	}
	e.priorityRound()
	// Float one restricted W and one restricted R through the real mana wheel.
	tapForRestrictedBatch(t, e, whiteID, "CantPayGenericCosts")
	tapForRestrictedBatch(t, e, redID, "CantPayGenericCosts")
	batches := restrictedBatchesOf(e, 0, "CantPayGenericCosts")
	if len(batches) != 2 || state.ManaSlot(batches[0].Color) != state.MW || state.ManaSlot(batches[1].Color) != state.MR {
		t.Fatalf("precondition: floating CantPayGenericCosts batches = %+v, want one W and one R", batches)
	}
	e.pending = nil
	e.priorityRound()
	return e, gid
}

// TestCastableRestrictedGenericBatchCannotPayTwobridGenericFace pins the
// over-offer the unresolved-cost availability read produced: four graveyard
// cards leave two of the three generic faces unpaid by delve, and the only
// floating mana is the W/R CantPayGenericCosts batch, which cannot pay them
// (nor the B/G/U coloured faces). There is no legal payment, so the offer
// gate must withhold -- exactly what the old composed CostPayable answered.
func TestCastableRestrictedGenericBatchCannotPayTwobridGenericFace(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, gid := twobridRestrictedEngine(t, reg, 4)
	cost := e.parseCost("2B 2G 2U")
	mana := cost
	mana.Generic -= e.delveCredit(0, gid, mana.Generic, false)
	if pay.CostPayable(asPayer(e), 0, gid, false, mana) {
		t.Fatal("precondition: the old composed gate already admits this cost, so the probe would not exercise the face walk")
	}
	if e.castable(0, gid, cost, false) {
		t.Fatalf("castable offered a twobrid+delve cast whose generic faces need a CantPayGenericCosts batch the announcement asks refuse")
	}
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == gid {
			t.Fatalf("the walk offered the unpayable twobrid cast: %+v", e.legalActions(0))
		}
	}
}

// TestCastablePricedTwobridDelveMirrorsCastable pins the two pricing modes to
// one primitive: with six graveyard cards and an empty pool every twobrid pip
// can announce its generic face and delve pays all six, so castable offers the
// cast. The potential-action walk's priced mode must offer it too; composing
// the delve credit against the unresolved (generic 0) cost withheld it.
func TestCastablePricedTwobridDelveMirrorsCastable(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, gid := twobridDelveEngine(t, reg, 6, "")
	cost := e.parseCost("2B 2G 2U")
	if !e.castable(0, gid, cost, false) {
		t.Fatal("precondition: the real walk does not offer the fully delve-funded twobrid cast")
	}
	if !e.castablePriced(0, gid, cost, false, state.Mana{}) {
		t.Fatal("the potential-action walk withheld a cast the real walk offers")
	}
}
