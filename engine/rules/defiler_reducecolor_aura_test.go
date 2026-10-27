// defiler_reducecolor_aura_test.go — a targeted green permanent spell (an
// Aura) that only the Defiler's reduction makes affordable: the CR 601.2c
// per-candidate target filter must price the answered reduction, or it drops
// every target and aborts the cast the offer gate promised.

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const defilerAuraSrc = "Name:Verdant Mantle\nManaCost:3 G G\nTypes:Enchantment Aura\nK:Enchant creature\n" +
	"A:SP$ Attach | Cost$ 3 G G | ValidTgts$ Creature | AILogic$ Pump\nOracle:x\n"

func TestDefilerAuraTargetFilterPricesTheReduction(t *testing.T) {
	t.Parallel()
	e, cfg, spell := defilerBoard(t, 1775, []string{"Defiler of Vigor"}, defilerAuraSrc, "Verdant Mantle", "CCCG")
	if defilerViews(e, spell) != 1 {
		t.Fatalf("precondition: the ReduceColor$ static does not apply to the Aura")
	}
	// {3}{G}{G} against {3}{G}: only the Defiler's trade makes it castable.
	submitChoices(t, e, castModeOption(t, e, spell, ""))
	defilerAnswer(t, e, true)
	if d := e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the target ask after the yes (not an abort), got %+v", d)
	}
	defilerSettle(t, e)
	defilerOnBattlefield(t, e, spell)
	if got := e.G.Players[0].Life; got != 18 {
		t.Fatalf("life = %d, want 18 (2 paid)", got)
	}
	if p := e.G.Players[0].Pool; p != (state.Mana{}) {
		t.Fatalf("pool = %v, want empty", p)
	}
	replayCheck(t, e, cfg)
}
