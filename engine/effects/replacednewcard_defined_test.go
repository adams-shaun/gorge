package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestReplacedNewCardTypeGate pins Semester's End's Defined$ spelling: a
// zone-change replacement's body names ReplacedNewCard.<Type> for the entering
// permanent, and the type after the dot is the whole read. Spark Double's
// clone rider uses the same spelling but is classified before it reaches the
// defined path, so this test covers the ordinary replacement.
func TestReplacedNewCardTypeGate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	h := newHost(t, 2)
	src := putOnBattlefield(t, h.g, reg, "Ravenous Slime", 0)
	bear := putOnBattlefield(t, h.g, reg, "Grizzly Bears", 1)
	c := &Ctx{Source: src.ID, Controller: 0}
	c.Repl.Replaced = bear.ID

	got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "ReplacedNewCard.Creature"}})
	if len(got) != 1 || got[0].Obj != bear.ID {
		t.Fatalf("ReplacedNewCard.Creature = %+v, want the entering bear %d", got, bear.ID)
	}
	if got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "ReplacedNewCard.Planeswalker"}}); len(got) != 0 {
		t.Fatalf("ReplacedNewCard.Planeswalker = %+v, want none for a creature", got)
	}
}
