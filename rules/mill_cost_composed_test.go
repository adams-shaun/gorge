package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// This file is the r1-review regression for the Mill<N> cost component
// (task agent-20260918T230554Z-2b1a0e21). Two defects:
//
//  1. A composed cost carrying several Mill parts (Mill<1> Mill<1>) was
//     priced one part at a time against the same FULL library, so with one
//     card it was offered; payment then milled that card and returned from
//     the second part without tapping the source or producing mana. The
//     parts must be summed (libraryCoversMill) and the payment must never
//     return part-way.
//
//  2. ParseCost now recognises Mill<N> globally, but only the mana-ability
//     path executed it. An ordinary activated ability with a Mill cost
//     (Sinister Concoction, Rot Farm Skeleton) was offered and paid without
//     milling. The ordinary cast/activation gate (nonManaCastable) and
//     payment (payMillCostParts) must handle it too.

const composedMillManaScript = "Name:Composed Mill Rock\nManaCost:0\nTypes:Artifact\n" +
	"A:AB$ Mana | Cost$ T Mill<1> Mill<1> | Produced$ C | SpellDescription$ Add {C}.\n" +
	"Oracle:x\n"

// millFodderCard is the filler card smallLibraryEngine stocks the library with.
func millFodderCard(t testing.TB) *cards.Card {
	t.Helper()
	return card(t, "Name:Mill Fodder\nTypes:Sorcery\nOracle:x\n")
}

// smallLibraryEngine is handEngine with the seat-0 library replaced by n
// anonymous cards, so a Mill requirement can be made exactly short or exactly
// met.
func smallLibraryEngine(t *testing.T, n int, hand ...*cards.Card) *Engine {
	t.Helper()
	e := handEngine(t, hand...)
	var ids []state.ObjID
	for i := 0; i < n; i++ {
		o := e.G.AddObject(millFodderCard(t), 0)
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZLibrary, 0, ids)
	return e
}

// TestComposedMillManaAbilityIsWithheldWhenLibraryCannotCoverEveryPart pins
// finding 1 in full: Mill<1> Mill<1> with a single card in the library is NOT
// payable (both parts draw from the same library in sequence), and resolving
// it anyway must be a total no-op -- the old code milled that one card and
// returned from the second part without tapping the source or producing mana,
// a cost paid in part.
func TestComposedMillManaAbilityIsWithheldWhenLibraryCannotCoverEveryPart(t *testing.T) {
	e := smallLibraryEngine(t, 1, card(t, composedMillManaScript))
	src := onBoard(t, e, 0, composedMillManaScript)
	o := e.G.Obj(src)
	if o == nil || len(o.Face().Abilities) == 0 {
		t.Fatal("precondition: composed Mill rock has no ability")
	}
	ma := o.Face().Abilities[0]
	parsed := ParseCost(ma.Params["Cost"])
	if len(parsed.Mill) != 2 || parsed.Mill[0].N != 1 || parsed.Mill[1].N != 1 {
		t.Fatalf("precondition: script did not parse as two Mill<1> parts: %+v", parsed.Mill)
	}
	lib := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	if len(lib) != 1 {
		t.Fatalf("precondition: library has %d cards, want 1", len(lib))
	}
	if e.manaAbilityPayable(0, src, ma) {
		t.Fatal("a Mill<1> Mill<1> mana ability must not be offered with one library card")
	}

	// Even reached directly, the resolution is a total no-op: no card mills,
	// no mana enters the pool, and the source never taps (the old part-at-a-
	// time payment milled the card and then returned early).
	e.resolveManaAbility(0, src, ma, false)
	if e.G.Obj(lib[0]).Zone != state.ZLibrary {
		t.Fatalf("partial Mill cost was paid: library card moved to %s", e.G.Obj(lib[0]).Zone)
	}
	if e.G.Players[0].Pool[state.MC] != 0 {
		t.Fatalf("unpayable composed Mill ability produced mana: pool=%+v", e.G.Players[0].Pool)
	}
	if e.G.Obj(src).Tapped {
		t.Fatal("unpayable composed Mill ability tapped its source")
	}

	// Positive control: two cards make the same composed cost payable and the
	// full cost is paid -- so the failing assertions above are not passing
	// because the ability is broken.
	e2 := smallLibraryEngine(t, 2, card(t, composedMillManaScript))
	src2 := onBoard(t, e2, 0, composedMillManaScript)
	ma2 := e2.G.Obj(src2).Face().Abilities[0]
	if !e2.manaAbilityPayable(0, src2, ma2) {
		t.Fatal("precondition: Mill<1> Mill<1> must be payable with two library cards")
	}
	lib2 := append([]state.ObjID(nil), e2.G.Zone(state.ZLibrary, 0)...)
	e2.resolveManaAbility(0, src2, ma2, false)
	for _, id := range lib2 {
		if e2.G.Obj(id).Zone != state.ZGraveyard {
			t.Fatalf("library card %v not milled: zone=%s", id, e2.G.Obj(id).Zone)
		}
	}
	if e2.G.Players[0].Pool[state.MC] != 1 || !e2.G.Obj(src2).Tapped {
		t.Fatalf("composed Mill ability paid only in part: pool=%+v tapped=%v", e2.G.Players[0].Pool, e2.G.Obj(src2).Tapped)
	}
}

// TestSpellMillCostMillsOnPayment covers the other ordinary payment branch the
// r1 review named (a spell, not an ability): casting a spell whose additional
// Cost$ carries Mill<1> really mills the top card as its cost.
func TestSpellMillCostMillsOnPayment(t *testing.T) {
	e := smallLibraryEngine(t, 2, card(t, "Name:Mill Spell\nManaCost:0\nTypes:Sorcery\nA:SP$ Draw | Cost$ Mill<1> | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n"))
	id := e.G.Zone(state.ZHand, 0)[0]
	lib := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	if len(lib) != 2 {
		t.Fatalf("precondition: library has %d cards, want 2", len(lib))
	}
	castMode(t, e, id, "")
	finishCast(t, e, id)
	if e.G.Obj(lib[0]).Zone != state.ZGraveyard {
		t.Fatalf("spell Mill<1> cost did not mill the top library card: zone=%s", e.G.Obj(lib[0]).Zone)
	}
}

// TestOrdinaryAbilityMillCostFailsClosedWhenLibraryIsShort pins finding 2's
// offer side on a real corpus carrier: Rot Farm Skeleton's
// "{2}{B}{G}, Mill four cards" ability is a NON-mana activated ability. With
// three library cards it must not be offered at all (before the fix, ParseCost
// read the Mill part but the ordinary gate ignored it, so the ability was
// offered and then never milled).
func TestOrdinaryAbilityMillCostFailsClosedWhenLibraryIsShort(t *testing.T) {
	skeleton, ok := testutil.CorpusRegistry(t).Lookup("Rot Farm Skeleton")
	if !ok {
		t.Fatal("corpus missing Rot Farm Skeleton")
	}
	e := smallLibraryEngine(t, 3, skeleton)
	id := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	if e.G.Obj(id).Zone != state.ZGraveyard {
		t.Fatalf("precondition: Rot Farm Skeleton is in %s, want graveyard", e.G.Obj(id).Zone)
	}
	// Fund the {2}{B}{G} so payability turns only on the mill requirement.
	e.G.Players[0].Pool[state.MC] = 2
	e.G.Players[0].Pool[state.MB] = 1
	e.G.Players[0].Pool[state.MG] = 1
	e.priorityRound()
	if _, offered := findAbilityOption(e, id, 0); offered {
		t.Fatalf("Rot Farm Skeleton must not be offered with 3 library cards (needs 4): %+v", e.Pending().Options)
	}

	// Positive control: a fourth library card makes it offered.
	e2 := smallLibraryEngine(t, 4, skeleton)
	id2 := e2.G.Zone(state.ZHand, 0)[0]
	e2.emit(events.Event{Kind: events.MoveZone, Obj: id2, From: state.ZHand, To: state.ZGraveyard})
	e2.G.Players[0].Pool[state.MC] = 2
	e2.G.Players[0].Pool[state.MB] = 1
	e2.G.Players[0].Pool[state.MG] = 1
	e2.priorityRound()
	if _, offered := findAbilityOption(e2, id2, 0); !offered {
		t.Fatalf("precondition: Rot Farm Skeleton must be offered with 4 library cards: %+v", e2.Pending().Options)
	}
}

// TestOrdinaryAbilityMillCostMillsBeforeResolving pins finding 2's payment
// side: activating Rot Farm Skeleton's ability really mills its four library
// cards as part of paying the cost.
func TestOrdinaryAbilityMillCostMillsBeforeResolving(t *testing.T) {
	skeleton, ok := testutil.CorpusRegistry(t).Lookup("Rot Farm Skeleton")
	if !ok {
		t.Fatal("corpus missing Rot Farm Skeleton")
	}
	e := smallLibraryEngine(t, 4, skeleton)
	id := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	lib := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	if len(lib) != 4 {
		t.Fatalf("precondition: library has %d cards, want 4", len(lib))
	}
	e.G.Players[0].Pool[state.MC] = 2
	e.G.Players[0].Pool[state.MB] = 1
	e.G.Players[0].Pool[state.MG] = 1
	e.priorityRound()
	opt, offered := findAbilityOption(e, id, 0)
	if !offered {
		t.Fatalf("Rot Farm Skeleton not offered with 4 library cards: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	finishCast(t, e, id)
	for _, cardID := range lib {
		if e.G.Obj(cardID).Zone != state.ZGraveyard {
			t.Fatalf("library card %v not milled to the graveyard: zone=%s", cardID, e.G.Obj(cardID).Zone)
		}
	}
	if e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("Rot Farm Skeleton did not resolve back to the battlefield: zone=%s", e.G.Obj(id).Zone)
	}
}
