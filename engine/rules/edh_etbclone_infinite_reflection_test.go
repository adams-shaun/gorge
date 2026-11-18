package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestEDHETBCloneRidersInfiniteReflectionEntry: with Infinite Reflection
// enchanting Serra Angel, a nontoken creature its controller casts enters as
// a copy of Serra Angel (the foreign-entry ETBReplacement:Copy, CR 614.1c,
// 707.2); a token creature entering under the same controller does not
// (Creature.!token).
func TestEDHETBCloneRidersInfiniteReflectionEntry(t *testing.T) {
	t.Parallel()
	e := goadGrantEngine(t, corpusAlternativeCard(t, "Grizzly Bears"), corpusAlternativeCard(t, "Infinite Reflection"))
	angel := onBoardReadyCard(t, e, 0, corpusAlternativeCard(t, "Serra Angel"))
	aura := lossAttach(t, e, "Infinite Reflection", 0, angel)
	passUntilStackEmpty(t, e, 8)
	if o := e.G.Obj(aura); o.Zone != state.ZBattlefield || o.AttachedTo != angel {
		t.Fatalf("Infinite Reflection is in %s attached to %d, want the battlefield on the Angel", o.Zone, o.AttachedTo)
	}
	if o := e.G.Obj(angel); o.Face().Name != "Serra Angel" {
		t.Fatalf("the enchanted creature is %q, want Serra Angel", o.Face().Name)
	}

	bears := e.G.Zone(state.ZHand, 0)[0]
	if e.G.Obj(bears).Face().Name != "Grizzly Bears" {
		t.Fatalf("hand card is %q, want Grizzly Bears", e.G.Obj(bears).Face().Name)
	}
	e.G.Players[0].Pool[state.MG] = 1
	e.G.Players[0].Pool[state.MC] = 1
	castMode(t, e, bears, "")
	passUntilStackEmpty(t, e, 8)
	o := e.G.Obj(bears)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Grizzly Bears = %+v, want it on the battlefield", o)
	}
	if o.Face().Name != "Serra Angel" || e.Power(bears) != 4 || e.Toughness(bears) != 4 {
		t.Fatalf("the cast creature entered as %q %d/%d, want a 4/4 Serra Angel copy",
			o.Face().Name, e.Power(bears), e.Toughness(bears))
	}

	tok := putToken(t, e, 0, "Name:Soldier\nTypes:Creature Soldier\nPT:1/1", state.ZBattlefield)
	passUntilStackEmpty(t, e, 8)
	if to := e.G.Obj(tok); to == nil || !to.IsToken || to.Zone != state.ZBattlefield || to.Face().Name != "Soldier" {
		t.Fatalf("token = %+v, want an uncopied Soldier token on the battlefield", to)
	}
}
