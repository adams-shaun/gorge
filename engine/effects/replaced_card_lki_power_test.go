package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestReplacedCardLKIPower pins Ravenous Slime's body: the opponent's dying
// 1/1 carrying a +1/+1 counter (power 2) is exiled and the Slime gets +1/+1
// counters equal to that creature's power as it was on the battlefield
// (X:ReplacedCardLKI$CardPower). The count reads the power rules captured when
// the replacement matched (Ctx.Repl.ReplacedPower), because the body's own move
// to exile has already cleared the creature's counters by the time PutCounter
// evaluates X.
func TestReplacedCardLKIPower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	slimeCard, ok := reg.Lookup("Ravenous Slime")
	if !ok {
		t.Fatal("corpus has no Ravenous Slime")
	}
	face := slimeCard.Faces[0]
	exile := svarSA(t, face, "Exile")

	h := newHost(t, 2)
	slime := putOnBattlefield(t, h.g, reg, "Ravenous Slime", 0)
	victim := putOnBattlefield(t, h.g, reg, "Llanowar Elves", 1)
	victim.Counters = []state.Counter{{Kind: "P1P1", N: 1}}
	if h.Power(victim.ID) != 2 {
		t.Fatalf("precondition: Llanowar Elves with a +1/+1 counter has power %d, want 2", h.Power(victim.ID))
	}
	if n := len(slime.Counters); n != 0 {
		t.Fatalf("precondition: Slime starts with %d counters", n)
	}

	c := &Ctx{Source: slime.ID, Controller: 0, SVars: face.SVars}
	c.Repl.Replaced = victim.ID
	c.Repl.ReplacedPower, c.Repl.ReplacedToughness, c.Repl.ReplacedPTValid = h.Power(victim.ID), h.Toughness(victim.ID), true
	Resolve(h, c, exile)

	if z := h.g.Obj(victim.ID).Zone; z != state.ZExile {
		t.Fatalf("the replaced creature is in zone %v, want exile", z)
	}
	got := int32(0)
	for _, k := range h.g.Obj(slime.ID).Counters {
		if k.Kind == "P1P1" {
			got += k.N
		}
	}
	if got != 2 {
		t.Fatalf("Ravenous Slime has %d +1/+1 counters, want 2 (the dying creature's power)", got)
	}
}
