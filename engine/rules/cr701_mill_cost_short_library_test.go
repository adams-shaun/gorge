package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 701.17b: "the player can't pay a cost that includes milling a number of
// cards greater than the number of cards in their library" (CR 118.3). Each
// of the three Mill<N> payment gates -- the cast/activation offer
// (pay.NonManaCastableP), the mana-ability offer (pay.ManaCostPayableFull)
// and the UnlessCost$ offer (pay.UnlessCostPayable) -- must refuse a mill
// cost the library cannot cover in full, and admit one it covers exactly.

// TestCR701_17bMillCostShortLibraryNotOffered: Rot Farm Skeleton's
// "{2}{B}{G}, Mill four cards: Return this card from your graveyard to the
// battlefield" is not offered with three library cards.
func TestCR701_17bMillCostShortLibraryNotOffered(t *testing.T) {
	t.Parallel()
	skeleton, ok := testutil.CorpusRegistry(t).Lookup("Rot Farm Skeleton")
	if !ok {
		t.Fatal("corpus missing Rot Farm Skeleton")
	}
	e := smallLibraryEngine(t, 3, skeleton)
	id := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	// Fund the {2}{B}{G} so the offer turns only on the mill requirement.
	e.G.Players[0].Pool[state.MC] = 2
	e.G.Players[0].Pool[state.MB] = 1
	e.G.Players[0].Pool[state.MG] = 1
	e.priorityRound()
	if _, offered := findAbilityOption(e, id, 0); offered {
		t.Fatalf("CR 701.17b Rot Farm Skeleton seq %d: Mill<4> ability offered with 3 library cards, want it withheld", len(e.L.Events))
	}
}

// TestCR701_17bMillManaAbilityEmptyLibraryUnpayable: Millikin's
// "{T}, Mill a card: Add {C}" is not payable from an empty library, and a
// composed Mill<1> Mill<1> mana ability needs two library cards, not one.
func TestCR701_17bMillManaAbilityEmptyLibraryUnpayable(t *testing.T) {
	t.Parallel()
	millikin, ok := testutil.CorpusRegistry(t).Lookup("Millikin")
	if !ok {
		t.Fatal("corpus missing Millikin")
	}
	e := smallLibraryEngine(t, 0, millikin)
	src := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZHand, To: state.ZBattlefield})
	e.G.Obj(src).SummonSick = false
	ma := millikin.Faces[0].Abilities[0]
	if p := ParseCost(ma.Params["Cost"]); ma.API != "Mana" || len(p.Mill) != 1 || p.Mill[0].N != 1 {
		t.Fatalf("precondition: Millikin ability is not the Mill<1> mana ability: %+v", ma)
	}
	if e.manaAbilityPayable(0, src, ma) {
		t.Errorf("CR 701.17b Millikin seq %d: Mill<1> mana ability payable with an empty library", len(e.L.Events))
	}

	c := smallLibraryEngine(t, 1, card(t, composedMillManaScript))
	rock := onBoard(t, c, 0, composedMillManaScript)
	if c.manaAbilityPayable(0, rock, c.G.Obj(rock).Face().Abilities[0]) {
		t.Errorf("CR 701.17b composed Mill rock seq %d: Mill<1> Mill<1> payable with one library card, want the summed 2 required", len(c.L.Events))
	}
	c2 := smallLibraryEngine(t, 2, card(t, composedMillManaScript))
	rock2 := onBoard(t, c2, 0, composedMillManaScript)
	if !c2.manaAbilityPayable(0, rock2, c2.G.Obj(rock2).Face().Abilities[0]) {
		t.Errorf("CR 701.17b composed Mill rock seq %d: Mill<1> Mill<1> unpayable with exactly two library cards", len(c2.L.Events))
	}
}

// TestCR701_17bDeepSpawnShortLibraryCannotPay: Deep Spawn's "sacrifice
// Deep Spawn unless you mill two cards" with one library card offers no Pay
// option, so Deep Spawn is sacrificed and the library card stays put.
func TestCR701_17bDeepSpawnShortLibraryCannotPay(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, 1} {
		e, spawn := deepSpawnEngine(t, n)
		lib := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
		e.Advance()
		e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
		e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
		for i := 0; i < 100 && e.G.Obj(spawn).Zone == state.ZBattlefield; i++ {
			d := e.Pending()
			if d == nil {
				t.Fatalf("CR 701.17b Deep Spawn lib=%d seq %d: stalled with no pending decision", n, len(e.L.Events))
			}
			if d.Kind == decision.KModes && d.ResumeKind == "unless_pay" {
				if len(d.Options) != 1 {
					t.Fatalf("CR 701.17b Deep Spawn lib=%d seq %d: unless ask offers %d options (first %q), want only the decline", n, len(e.L.Events), len(d.Options), d.Options[0].Label)
				}
				submitChoices(t, e, d.Options[0].Index)
				continue
			}
			if d.Kind != decision.KPriority {
				t.Fatalf("CR 701.17b Deep Spawn lib=%d seq %d: unexpected %v", n, len(e.L.Events), d.Kind)
			}
			castFirst(t, e, "pass")
		}
		if z := e.G.Obj(spawn).Zone; z != state.ZGraveyard {
			t.Errorf("CR 701.17b Deep Spawn lib=%d seq %d: zone = %s, want Graveyard (the Mill<2> cost cannot be paid)", n, len(e.L.Events), z)
		}
		if moved := milledToGraveyard(e, lib); len(moved) != 0 {
			t.Errorf("CR 701.17b Deep Spawn lib=%d seq %d: milled %v, want nothing (no partial payment)", n, len(e.L.Events), moved)
		}
	}
}
