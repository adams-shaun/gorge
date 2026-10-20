package rules

// CR 700.5: "A player's devotion to [color 1] and [color 2] is equal to the
// number of mana symbols among the mana costs of permanents that player
// controls that are [color 1], [color 2], or both colors." A hybrid {R/G}
// symbol is ONE symbol that is both colours, so it counts once toward
// devotion to red and green, never twice. Xenagos, God of Revels is the
// corpus repro: its RemoveType$ Creature static is gated on
// Count$DevotionDual.Red.Green LT7, so a double-counted hybrid turns the god
// into a creature at a CR devotion of six.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestXenagosDevotionDualCountsHybridOnce: Xenagos ({3}{R}{G}: 2) plus two
// Burning-Tree Emissaries ({R/G}{R/G}: 2 each) is CR devotion 6 to red and
// green -- below seven, so Xenagos is NOT a creature. Adding one {R}
// permanent brings it to exactly 7 and Xenagos IS a 6/5 creature (the guard
// half: the fix must not undercount).
func TestXenagosDevotionDualCountsHybridOnce(t *testing.T) {
	xen := corpusCard(t, "Xenagos, God of Revels")
	f := xen.Faces[0]
	if got := f.SVars["X"]; got != "Count$DevotionDual.Red.Green" {
		t.Fatalf("corpus Xenagos SVar:X = %q, want Count$DevotionDual.Red.Green", got)
	}
	gated := false
	for i := range f.Statics {
		s := &f.Statics[i]
		if s.Mode == "Continuous" && s.Params["RemoveType"] == "Creature" &&
			s.Params["CheckSVar"] == "X" && s.Params["SVarCompare"] == "LT7" {
			gated = true
		}
	}
	if !gated {
		t.Fatalf("corpus Xenagos carries no RemoveType$ Creature static gated on X LT7: %+v", f.Statics)
	}
	bte := corpusCard(t, "Burning-Tree Emissary")
	if got := bte.Faces[0].ManaCost; got != "RG RG" {
		t.Fatalf("corpus Burning-Tree Emissary ManaCost = %q, want %q", got, "RG RG")
	}

	e := layerEngine(t)
	xenID := onBoardCard(t, e, 0, xen)
	for i := 0; i < 2; i++ {
		id := onBoardCard(t, e, 0, bte)
		if e.G.Obj(id).Zone != state.ZBattlefield {
			t.Fatalf("precondition broken: Emissary %d not on the battlefield", i)
		}
	}
	ctx := &effects.Ctx{Controller: 0, Source: xenID}
	body := f.SVars["X"]

	// Xenagos's {R}{G} is 2; each Emissary's two {R/G} symbols are 2 more.
	if n, ok := effects.EvalCountOK(e, ctx, body); !ok || n != 6 {
		t.Fatalf("devotion to red and green, Xenagos + 2 Emissaries = (%d, %v), want (6, true): each {R/G} hybrid is ONE symbol that is both colours (CR 700.5)", n, ok)
	}
	if ty := e.Derived(xenID).Types; hasTypeWord(ty, "Creature") {
		t.Fatalf("devotion-6 Xenagos types include Creature, want none (types %v)", ty)
	}

	// Guard: one more {R} permanent is devotion 7 -- the LT7 gate releases.
	onBoardCard(t, e, 0, card(t, "Name:Red Pip Goblin\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"))
	if n, ok := effects.EvalCountOK(e, ctx, body); !ok || n != 7 {
		t.Fatalf("devotion to red and green after one {R} more = (%d, %v), want (7, true)", n, ok)
	}
	if ty := e.Derived(xenID).Types; !hasTypeWord(ty, "Creature") {
		t.Fatalf("devotion-7 Xenagos types lack Creature (types %v)", ty)
	}
	if p, tg := e.Power(xenID), e.Toughness(xenID); p != 6 || tg != 5 {
		t.Fatalf("devotion-7 Xenagos P/T = %d/%d, want 6/5", p, tg)
	}
}
