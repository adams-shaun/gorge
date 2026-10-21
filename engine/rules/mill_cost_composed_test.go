package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// This file is the regression suite for the Mill<N> cost component
// (task agent-20260918T230554Z-2b1a0e21). Ordinary activated abilities and
// spells must settle their parsed Mill cost parts as real zone moves.

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

// TestSpellMillCostMillsOnPayment covers the other ordinary payment branch the
// r1 review named (a spell, not an ability): casting a spell whose additional
// Cost$ carries Mill<1> really mills the top card as its cost.
func TestSpellMillCostMillsOnPayment(t *testing.T) {
	t.Parallel()
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

// TestOrdinaryAbilityMillCostMillsBeforeResolving pins finding 2's payment
// side: activating Rot Farm Skeleton's ability really mills its four library
// cards as part of paying the cost.
func TestOrdinaryAbilityMillCostMillsBeforeResolving(t *testing.T) {
	t.Parallel()
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
