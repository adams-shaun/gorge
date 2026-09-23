package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// TestLibrarySearchLimitsMaxRevealedToTheTopWindow proves that a limited
// library look does not turn into an unrestricted search. Both cards are
// deliberately eligible and in the library, but only the first is inside the
// one-card look window (the second is outside it).
func TestLibrarySearchLimitsMaxRevealedToTheTopWindow(t *testing.T) {
	h := newHost(t, 2)
	source := mkCard(t, "Name:Limited Search\nTypes:Sorcery\nOracle:x\n")
	first := mkCard(t, "Name:Top Card\nTypes:Creature\nPT:2/2\nOracle:x\n")
	second := mkCard(t, "Name:After Window\nTypes:Creature\nPT:2/2\nOracle:x\n")
	src := h.g.AddObject(source, 0)
	a := h.g.AddObject(first, 0)
	b := h.g.AddObject(second, 0)
	h.g.SetZone(state.ZHand, 0, []state.ObjID{src.ID})
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{a.ID, b.ID})
	h.g.Obj(src.ID).Zone = state.ZHand
	h.g.Obj(a.ID).Zone = state.ZLibrary
	h.g.Obj(b.ID).Zone = state.ZLibrary
	lib := h.g.Zone(state.ZLibrary, 0)
	if a.ID == b.ID || len(lib) != 2 || lib[0] != a.ID || lib[1] != b.ID {
		t.Fatalf("precondition failed: ordered library window = %v, want [%d %d]", lib, a.ID, b.ID)
	}

	waiting := &suspendHost{}
	waiting.g = h.g
	sa := &cards.SA{API: "ChangeZone", Params: map[string]string{
		"Origin": "Library", "Destination": "Hand", "ChangeType": "Creature",
		"ChangeNum": "1", "MaxRevealed": "1",
	}}
	Resolve(waiting, &Ctx{Source: src.ID, Controller: 0}, sa)

	if waiting.asked == nil || waiting.asked.Kind != decision.KChoose {
		t.Fatalf("limited search did not pose a choice: %+v", waiting.asked)
	}
	if len(waiting.asked.Options) != 1 || waiting.asked.Options[0].Obj != a.ID {
		t.Fatalf("limited search offered %+v, want only top card %d", waiting.asked.Options, a.ID)
	}
	for _, opt := range waiting.asked.Options {
		if opt.Obj == b.ID {
			t.Fatalf("card outside MaxRevealed window was offered: %+v", waiting.asked.Options)
		}
	}
}
