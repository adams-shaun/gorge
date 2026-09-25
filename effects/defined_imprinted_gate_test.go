package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestDefinedImprintedKeepsTheExileGate pins the scoping the r2 review of
// cli-20260923T060000Z-rv2b-damagesource demanded: the ungated raw imprint
// read (Forge's getImprintedCards shape, no CR 607.2a zone gate) belongs ONLY
// to the damage-source and count consumers that need it (damage.go's
// damageSourceSpecTargets, count.go's refTargets Imprinted case). The shared
// definedSpec resolver keeps the CR 607.2a exile gate for its every ordinary
// Defined$ caller: an imprint association naming a BATTLEFIELD object is not
// in the pile -- CR 607.2a links the ability to the card only while it stays
// in exile -- and reverting the shared fallback must keep it that way.
func TestDefinedImprintedKeepsTheExileGate(t *testing.T) {
	h := newHost(t, 2)
	linked := h.g.AddObject(mkCard(t, "Name:Linked\nTypes:Creature\nOracle:x\n"), 0)
	src := h.g.AddObject(mkCard(t, "Name:Src\nTypes:Enchantment\nOracle:x\n"), 0)
	src.Imprinted = []state.ObjID{linked.ID}
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID, linked.ID})
	for _, id := range []state.ObjID{src.ID, linked.ID} {
		h.g.Obj(id).Zone = state.ZBattlefield
	}

	// Preconditions: the association names exactly one object and that object
	// IS on the battlefield -- the two facts the exile gate reads. An empty
	// gated pile below is only meaningful against this setup.
	if o := h.g.Obj(linked.ID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: linked card = %+v, want on the battlefield", o)
	}
	if len(src.Imprinted) != 1 || src.Imprinted[0] != linked.ID {
		t.Fatalf("precondition: src.Imprinted = %v, want [%d]", src.Imprinted, linked.ID)
	}
	c := &Ctx{Source: src.ID, Controller: 0}

	// Ordinary Defined$ Imprinted: the gated pile is EMPTY -- the only linked
	// card sits on the battlefield, outside CR 607.2a's exile gate.
	got, ok := definedSpec(h, c, "Imprinted")
	if !ok {
		t.Fatal("definedSpec Imprinted -> not recognised, want a recognised (ok) empty gated pile")
	}
	if len(got) != 0 {
		t.Fatalf("definedSpec Imprinted -> %v, want empty (the battlefield-linked card is not in the CR 607.2a pile)", got)
	}
	// The public Defined entry (what every ordinary effect resolves through)
	// agrees -- no raw fallback reintroduced there either.
	if got := Defined(h, c, sa(t, "SP$ X | Defined$ Imprinted")); len(got) != 0 {
		t.Fatalf("Defined$ Imprinted -> %v, want empty through the public Defined entry", got)
	}
	// The player selector is a separate raw association read: even this
	// battlefield-linked card names its current controller.
	if got, ok := definedSpec(h, c, "ImprintedController"); !ok || len(got) != 1 || got[0] != (state.Target{Player: 0, IsPlayer: true}) {
		t.Fatalf("definedSpec ImprintedController -> %v ok=%v, want controller 0", got, ok)
	}

	// The contrast that makes the gate meaningful: the DAMAGE-SOURCE reader
	// keeps its own scoped raw fallback and DOES resolve the
	// battlefield-linked association (Enchanter's Bane's shape, the corpus
	// carrier this whole sub-shape exists for).
	ts, ok := damageSourceSpecTargets(h, c, "Imprinted")
	if !ok || len(ts) != 1 || ts[0] != (state.Target{Obj: linked.ID}) {
		t.Fatalf("damageSourceSpecTargets Imprinted -> %v ok=%v, want the battlefield-linked [%d] through the scoped raw read", ts, ok, linked.ID)
	}
}
