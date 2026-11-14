package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestReplacedCardRefOnlyReadsPT pins the gate on the ReplacedCard /
// ReplacedCardLKI count refs: the corpus's only supported read there is the
// replaced card's P/T (Ravenous Slime's X:ReplacedCardLKI$CardPower, the
// value replCtx captured at match time -- 5 here, not the bear's live 2, so
// the captured path is the one that resolved). The mana reads the other
// corpus users carry -- Thunderous Velocipede's ReplacedCard$CardManaCost,
// Coin of Mastery's ReplacedCard$CastTotalManaSpent Artifact, Kalain's
// ReplacedCardLKI$CastTotalManaSpent Treasure -- keep failing closed to 0
// exactly as they did before the ref resolved at all.
func TestReplacedCardRefOnlyReadsPT(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	h := newHost(t, 2)
	src := putOnBattlefield(t, h.g, reg, "Ravenous Slime", 0)
	victim := putOnBattlefield(t, h.g, reg, "Grizzly Bears", 1)
	c := &Ctx{Source: src.ID, Controller: 0}
	c.Repl.Replaced = victim.ID
	c.Repl.ReplacedPower, c.Repl.ReplacedToughness, c.Repl.ReplacedPTValid = 5, 5, true

	if n, ok := evalRefProperty(h, c, "ReplacedCardLKI$CardPower", 0); !ok || n != 5 {
		t.Fatalf("ReplacedCardLKI$CardPower = %d,%v, want 5,true (the captured P/T read)", n, ok)
	}
	for _, expr := range []string{
		"ReplacedCard$CardManaCost",
		"ReplacedCard$CastTotalManaSpent Artifact",
		"ReplacedCardLKI$CastTotalManaSpent Treasure",
	} {
		if n, ok := evalRefProperty(h, c, expr, 0); n != 0 || ok {
			t.Fatalf("%s = %d,%v, want 0,false: only the P/T read of the replaced card is modelled", expr, n, ok)
		}
	}
}
