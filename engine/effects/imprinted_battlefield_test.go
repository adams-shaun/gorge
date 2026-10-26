package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestIsImprintedMatchesBattlefieldCandidateOnly pins the battlefield arm of
// the IsImprinted predicate: ImprintCards$ on a battlefield permanent (Winnowing)
// is readable by Card.IsImprinted while the card stays on the battlefield, but
// not after it leaves, and Defined$ Imprinted keeps its exile gate.
func TestIsImprintedMatchesBattlefieldCandidateOnly(t *testing.T) {
	h := newHost(t, 2)
	g := h.g
	src := g.AddObject(mkCard(t, "Name:Src\nTypes:Enchantment\nOracle:x\n"), 0)
	linked := g.AddObject(mkCard(t, "Name:Linked\nTypes:Creature\nOracle:x\n"), 0)
	bystander := g.AddObject(mkCard(t, "Name:Bystander\nTypes:Creature\nOracle:x\n"), 0)
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID, linked.ID, bystander.ID})
	for _, id := range []state.ObjID{src.ID, linked.ID, bystander.ID} {
		g.Obj(id).Zone = state.ZBattlefield
	}
	src, linked, bystander = g.Obj(src.ID), g.Obj(linked.ID), g.Obj(bystander.ID)
	src.Imprinted = []state.ObjID{linked.ID}
	if linked.Zone != state.ZBattlefield || linked.ID == bystander.ID || bystander.Zone != state.ZBattlefield {
		t.Fatalf("precondition: linked and bystander must be distinct battlefield cards (linked=%d/%v bystander=%d/%v)", linked.ID, linked.Zone, bystander.ID, bystander.Zone)
	}
	sc := SpecContext{You: 0, Source: src.ID}
	if !MatchesObjectCtx(g, "Card.IsImprinted", linked, sc) {
		t.Error("Card.IsImprinted must match a linked card on the battlefield")
	}
	if MatchesObjectCtx(g, "Card.IsImprinted", bystander, sc) {
		t.Error("Card.IsImprinted matched an unlinked battlefield bystander")
	}
	c := &Ctx{Source: src.ID, Controller: 0}
	if got, ok := definedSpec(h, c, "Imprinted"); !ok || len(got) != 0 {
		t.Errorf("definedSpec Imprinted -> %v ok=%v, want an empty exile-gated pile", got, ok)
	}

	g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID, bystander.ID})
	g.SetZone(state.ZGraveyard, 0, []state.ObjID{linked.ID})
	g.Obj(linked.ID).Zone = state.ZGraveyard
	linked = g.Obj(linked.ID)
	if linked.Zone != state.ZGraveyard {
		t.Fatal("precondition: linked card must have left the battlefield")
	}
	if MatchesObjectCtx(g, "Card.IsImprinted", linked, sc) {
		t.Error("Card.IsImprinted must stop matching once the linked card is in the graveyard")
	}
}
