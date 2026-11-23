package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTopOfLibraryCMCCombinesASplitCard: the mana value of a split card on top
// of the library is its halves combined (CR 709.4b, 202.3d), the number
// state.ManaValue gives, not the front face's alone (the atom harness found
// gorge 2 against the kernel's 8 for Push // Pull).
func TestTopOfLibraryCMCCombinesASplitCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c, ok := reg.Lookup("Boom")
	if !ok {
		t.Skip("corpus has no Boom // Bust")
	}
	o := g.AddObject(c, 0)
	g.SetZone(state.ZLibrary, 0, []state.ObjID{o.ID})
	g.Obj(o.ID).Zone = state.ZLibrary
	top := g.Obj(o.ID)
	front, combined := int(top.Face().ManaValue()), int(state.ManaValue(g, top))
	if front == combined {
		t.Fatalf("precondition: front face %d and combined %d mana value must differ", front, combined)
	}
	if got := int(EvalCount(h, &Ctx{Controller: 0}, "Count$TopOfLibraryCMC")); got != combined {
		t.Errorf("Count$TopOfLibraryCMC = %d, want the combined %d (front face %d)", got, combined, front)
	}
}

// TestBlockedBySourceUnboundSource: with no source bound, blockedBySource
// fails closed. An attacker's BlockedBy can hold the zero id (a blocker that
// left), which an unbound source of 0 must not match.
func TestBlockedBySourceUnboundSource(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears")
	id := bear.ID
	g.Obj(id).BlockedBy = []state.ObjID{0}
	g.NoteBlockers()
	if o := g.Obj(id); len(o.BlockedBy) != 1 || o.BlockedBy[0] != 0 {
		t.Fatalf("precondition: BlockedBy must hold the zero id, got %v", o.BlockedBy)
	}
	if blockHistoryMatches(wordBlockedBySource, "", g, g.Obj(id), &SpecContext{You: 0}) {
		t.Errorf("blockedBySource matched with no source bound")
	}
	if MatchesObjectCtx(g, "Creature.blockedBySource", g.Obj(id), SpecContext{You: 0}) {
		t.Errorf("Creature.blockedBySource matched with no source bound")
	}
}
