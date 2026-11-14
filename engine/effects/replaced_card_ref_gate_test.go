package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestReplacedCardRefOnlyReadsPT pins the split between the two refs. The
// ReplacedCardLKI ref reads only the replaced card's P/T (Ravenous Slime's
// X:ReplacedCardLKI$CardPower, the value replCtx captured at match time -- 5
// here, not the bear's live 2, so the captured path is the one that
// resolved); Kalain's ReplacedCardLKI$CastTotalManaSpent Treasure keeps
// failing closed to 0. The plain ReplacedCard ref is untouched by the
// capture: Thunderous Velocipede's ReplacedCard$CardManaCost still reads the
// replaced card's live mana value (Grizzly Bears, {1}{G}: 2), which the
// copy_entry_trigger_after_order_ask lockstep repro depends on.
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
	if n, ok := evalRefProperty(h, c, "ReplacedCard$CardPower", 0); !ok || n != 2 {
		t.Fatalf("ReplacedCard$CardPower = %d,%v, want 2,true (the live read, not the capture)", n, ok)
	}
	if n, ok := evalRefProperty(h, c, "ReplacedCard$CardManaCost", 0); !ok || n != 2 {
		t.Fatalf("ReplacedCard$CardManaCost = %d,%v, want 2,true (Velocipede's live mana value)", n, ok)
	}
	if n, ok := evalRefProperty(h, c, "ReplacedCardLKI$CastTotalManaSpent Treasure", 0); n != 0 || ok {
		t.Fatalf("ReplacedCardLKI$CastTotalManaSpent = %d,%v, want 0,false: only the P/T read of the LKI ref is modelled", n, ok)
	}
}
