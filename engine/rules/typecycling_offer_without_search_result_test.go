package rules

// CR 702.29e: "[Type]cycling [cost]" is "[Cost], Discard this card: Search
// your library for a [type] card, reveal it, and put it into your hand. Then
// shuffle your library." The search names no target, so the activation is
// legal whenever the cost is payable, whether or not the library holds a
// matching card (a failed search is a legal resolution, CR 701.17d). These
// tests pin the OFFER: basic landcycling is offered with a basic land in the
// library AND with none.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func absVisOfferCount(t *testing.T, e *Engine) int {
	t.Helper()
	n := 0
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" {
			n++
		}
	}
	return n
}

func nonbasicLibrary(t *testing.T, e *Engine) {
	t.Helper()
	beast := card(t, "Name:Test Beast\nTypes:Creature Beast\nOracle:x\n")
	var ids []state.ObjID
	for i := 0; i < 7; i++ {
		o := e.G.AddObject(beast, 0)
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZLibrary, 0, ids)
}

func TestAbsorbVisCycleOfferedWithBasicInLibrary(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Absorb Vis"))
	e.G.Players[0].Pool[state.MB] = 2
	if n := absVisOfferCount(t, e); n != 1 {
		t.Fatalf("basic landcycling with a basic land in the library: want 1 ability offer, got %d", n)
	}
}

func TestAbsorbVisCycleOfferedWithoutAnyBasicInLibrary(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Absorb Vis"))
	nonbasicLibrary(t, e)
	e.G.Players[0].Pool[state.MB] = 2
	if n := absVisOfferCount(t, e); n != 1 {
		t.Fatalf("basic landcycling with NO basic land in the library: want 1 ability offer, got %d", n)
	}
}
