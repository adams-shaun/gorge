package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Forge's PlayerProperty has ONE generic `controls<ValidCards>[_<cmp><n>]`
// rule: the text after "controls" is a ValidCards filter over the player's
// battlefield plus an optional count comparison (default GE1). The dotted
// controlsCard./controlsCreature./controlsPermanent. spellings are cases of
// that same rule, not separate grammar. This pins the generic spellings the
// corpus carries.
func TestPlayerControlsGenericProperty(t *testing.T) {
	t.Parallel()

	t.Run("controlsElf_EQ0", func(t *testing.T) {
		g := state.NewGame([]string{"a", "b", "c"})
		qualifierObject(t, g, 0, "Name:Elf Scout\nManaCost:G\nTypes:Creature Elf Scout\nPT:1/1\nOracle:x\n")
		qualifierObject(t, g, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		// Precondition: seat 0 controls an Elf and seat 1 controls none.
		if !MatchesPlayerSpec(g, "Player.controlsCreature.Elf", 0, 0) {
			t.Fatal("precondition: seat 0 must control an Elf")
		}
		if MatchesPlayerSpec(g, "Player.controlsCreature.Elf", 1, 0) {
			t.Fatal("precondition: seat 1 must control no Elf")
		}
		if MatchesPlayerSpec(g, "Player.controlsElf_EQ0", 0, 0) {
			t.Fatal("controlsElf_EQ0 matched seat 0, which controls an Elf")
		}
		if !MatchesPlayerSpec(g, "Player.controlsElf_EQ0", 1, 0) {
			t.Fatal("controlsElf_EQ0 missed seat 1, which controls no Elf")
		}
	})

	t.Run("controlsLand_GE8", func(t *testing.T) {
		g := state.NewGame([]string{"a", "b"})
		for i := 0; i < 8; i++ {
			qualifierObject(t, g, 0, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n")
		}
		for i := 0; i < 7; i++ {
			qualifierObject(t, g, 1, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
		}
		// Precondition: seat 0 has exactly 8 lands and seat 1 exactly 7, so
		// GE8 separates them.
		if got := len(g.Zone(state.ZBattlefield, 0)); got != 8 {
			t.Fatalf("precondition: seat 0 battlefield = %d, want 8", got)
		}
		if got := len(g.Zone(state.ZBattlefield, 1)); got != 7 {
			t.Fatalf("precondition: seat 1 battlefield = %d, want 7", got)
		}
		if !MatchesPlayerSpec(g, "Player.controlsLand_GE8", 0, 0) {
			t.Fatal("controlsLand_GE8 missed the seat with eight lands")
		}
		if MatchesPlayerSpec(g, "Player.controlsLand_GE8", 1, 0) {
			t.Fatal("controlsLand_GE8 matched the seat with seven lands")
		}
	})

	t.Run("controlsEnchantment", func(t *testing.T) {
		g := state.NewGame([]string{"a", "b"})
		qualifierObject(t, g, 0, "Name:Test Enchantment\nManaCost:1 W\nTypes:Enchantment\nOracle:x\n")
		// Precondition: only seat 0 controls an enchantment.
		if !MatchesPlayerSpec(g, "Player.controlsEnchantment", 0, 0) {
			t.Fatal("controlsEnchantment missed the seat controlling one (default GE1)")
		}
		if MatchesPlayerSpec(g, "Player.controlsEnchantment", 1, 0) {
			t.Fatal("controlsEnchantment matched the seat controlling none")
		}
	})

	t.Run("controlsLegendary.Creature_EQ0", func(t *testing.T) {
		g := state.NewGame([]string{"a", "b", "c"})
		qualifierObject(t, g, 0, "Name:Legend\nManaCost:1 G\nTypes:Legendary Creature Wolf\nPT:2/2\nOracle:x\n")
		qualifierObject(t, g, 1, "Name:Mortal Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		// Precondition: seat 0 controls a legendary creature and seat 1 only a
		// nonlegendary one.
		if !MatchesPlayerSpec(g, "Player.controlsLegendary.Creature", 0, 0) {
			t.Fatal("precondition: seat 0 must control a legendary creature")
		}
		if MatchesPlayerSpec(g, "Player.controlsLegendary.Creature", 1, 0) {
			t.Fatal("precondition: seat 1 must control no legendary creature")
		}
		if MatchesPlayerSpec(g, "Player.controlsLegendary.Creature_EQ0", 0, 0) {
			t.Fatal("controlsLegendary.Creature_EQ0 matched seat 0, which controls one")
		}
		if !MatchesPlayerSpec(g, "Player.controlsLegendary.Creature_EQ0", 1, 0) {
			t.Fatal("controlsLegendary.Creature_EQ0 missed seat 1, which controls none")
		}
	})

	t.Run("controlsCreature_EQX", func(t *testing.T) {
		g := state.NewGame([]string{"a", "b"})
		qualifierObject(t, g, 0, "Name:Only Creature\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		h := &fakeHost{g: g}
		c := &Ctx{Controller: 0, SVars: map[string]string{"X": "1"}}
		// Precondition: the literal count is meaningful on this board.
		if !MatchesPlayerSpec(g, "Player.controlsCreature_EQ1", 0, 0) {
			t.Fatal("precondition: seat 0 must control exactly one creature")
		}
		if !MatchesPlayerSpecWithSVars(h, c, "Player.controlsCreature_EQX", 0, 0) {
			t.Fatal("controlsCreature_EQX did not resolve X through the SVar path")
		}
		if !MatchesPlayerSpec(g, "Player.controlsCreature_EQ0", 1, 0) {
			t.Fatal("seat 1 controls no creature, so EQ0 must match it (sanity check)")
		}
	})
}
