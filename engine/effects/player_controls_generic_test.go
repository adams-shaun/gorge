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

	// A `+` clause whose controls<ValidCards> text contains a '.' is still a
	// qualifier on the candidate: the '.' belongs to the ValidCards filter
	// (`Legendary.Creature`, `Creature.Bear`), not to a second base separator.
	// Nicol Bolas, Dragon-God's ultimate is the corpus spelling.
	t.Run("Opponent plus dotted legendary and planeswalker", func(t *testing.T) {
		g := state.NewGame([]string{"a", "b", "c", "d"})
		qualifierObject(t, g, 1, "Name:Mortal Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		qualifierObject(t, g, 2, "Name:Legend\nManaCost:1 G\nTypes:Legendary Creature Wolf\nPT:2/2\nOracle:x\n")
		qualifierObject(t, g, 3, "Name:Test Walker\nManaCost:3\nTypes:Planeswalker\nOracle:x\n")
		const spec = "Player.Opponent+controlsLegendary.Creature_EQ0+controlsPlaneswalker_LE0"
		// Preconditions: seat 1 controls neither a legendary creature nor a
		// planeswalker; seat 2 controls a legendary creature and seat 3 a
		// planeswalker, so each half of the conjunction separates them.
		if MatchesPlayerSpec(g, "Player.controlsLegendary.Creature", 1, 0) {
			t.Fatal("precondition: seat 1 must control no legendary creature")
		}
		if !MatchesPlayerSpec(g, "Player.controlsLegendary.Creature", 2, 0) {
			t.Fatal("precondition: seat 2 must control a legendary creature")
		}
		if !MatchesPlayerSpec(g, "Player.controlsPlaneswalker", 3, 0) {
			t.Fatal("precondition: seat 3 must control a planeswalker")
		}
		if !MatchesPlayerSpec(g, spec, 1, 0) {
			t.Fatal("Opponent+controlsLegendary.Creature_EQ0+controlsPlaneswalker_LE0 missed the opponent controlling neither")
		}
		if MatchesPlayerSpec(g, spec, 2, 0) {
			t.Fatal("the legendary-creature clause was dropped: seat 2 controls one but matched")
		}
		if MatchesPlayerSpec(g, spec, 3, 0) {
			t.Fatal("the planeswalker clause was dropped: seat 3 controls one but matched")
		}
	})

	t.Run("Opponent plus dotted creature count", func(t *testing.T) {
		g := state.NewGame([]string{"a", "b"})
		qualifierObject(t, g, 1, "Name:Elf Scout\nManaCost:G\nTypes:Creature Elf Scout\nPT:1/1\nOracle:x\n")
		// Precondition: seat 1 controls exactly one Elf and seat 0 none.
		if !MatchesPlayerSpec(g, "Player.controlsCreature.Elf_EQ1", 1, 0) {
			t.Fatal("precondition: seat 1 must control exactly one Elf")
		}
		if MatchesPlayerSpec(g, "Player.controlsCreature.Elf_EQ1", 0, 0) {
			t.Fatal("precondition: seat 0 must control no Elf")
		}
		if !MatchesPlayerSpec(g, "Player.Opponent+controlsCreature.Elf", 1, 0) {
			t.Fatal("Opponent+controlsCreature.Elf missed the Elf's controller")
		}
		if !MatchesPlayerSpec(g, "Player.Opponent+controlsCreature.Elf_GE1", 1, 0) {
			t.Fatal("Opponent+controlsCreature.Elf_GE1 missed the Elf's controller")
		}
	})

	t.Run("Opponent plus dotted creature EQX", func(t *testing.T) {
		g := state.NewGame([]string{"a", "b"})
		qualifierObject(t, g, 1, "Name:Elf Scout\nManaCost:G\nTypes:Creature Elf Scout\nPT:1/1\nOracle:x\n")
		h := &fakeHost{g: g}
		const spec = "Player.Opponent+controlsCreature.Elf_EQX"
		// Precondition: X=1 is the literal count of seat 1's Elves.
		if !MatchesPlayerSpec(g, "Player.controlsCreature.Elf_EQ1", 1, 0) {
			t.Fatal("precondition: seat 1 must control exactly one Elf")
		}
		if !MatchesPlayerSpecWithSVars(h, &Ctx{Controller: 0, SVars: map[string]string{"X": "1"}}, spec, 1, 0) {
			t.Fatal("Opponent+controlsCreature.Elf_EQX did not resolve X through the SVar path")
		}
		if MatchesPlayerSpecWithSVars(h, &Ctx{Controller: 0, SVars: map[string]string{"X": "2"}}, spec, 1, 0) {
			t.Fatal("Opponent+controlsCreature.Elf_EQX matched with X=2 against one Elf")
		}
		if MatchesPlayerSpecWithSVars(h, &Ctx{Controller: 0, SVars: map[string]string{"X": "0"}}, spec, 0, 0) {
			t.Fatal("the Opponent base was dropped: seat 0 is not an opponent of itself")
		}
	})

	t.Run("Opponent plus dotted attacking and enchanted rider", func(t *testing.T) {
		// The Commander curses' raw spelling: the ValidCards text is
		// `Creature.attacking Player.EnchantedBy`, so its first '.' is inside
		// the object filter too. The engine canonicalises the real scripts to
		// the TriggeredAttackingPlayer form, but the filter itself must read
		// the raw spelling as the same property.
		g := state.NewGame([]string{"attacker", "curse controller", "enchanted"})
		curse := qualifierObject(t, g, 1, "Name:Test Curse\nManaCost:1 R\nTypes:Enchantment Aura Curse\nOracle:x\n")
		curse.HasAttachedPlayer, curse.AttachedPlayer = true, 2
		bear := qualifierObject(t, g, 0, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		bear.IsAttacking, bear.Attacking = true, 2
		const spec = "Player.Opponent+controlsCreature.attacking Player.EnchantedBy"
		// Precondition: seat 0's Bear attacks the seat the curse enchants.
		if !curse.HasAttachedPlayer || curse.AttachedPlayer != 2 || !bear.IsAttacking || bear.Attacking != 2 {
			t.Fatalf("precondition: curse enchants %d, Bear attacks %d", curse.AttachedPlayer, bear.Attacking)
		}
		if !MatchesPlayerSpecFrom(g, spec, 0, 1, curse.ID) {
			t.Fatal("the attacking opponent did not match the raw curse spelling")
		}
		if MatchesPlayerSpecFrom(g, spec, 1, 1, curse.ID) {
			t.Fatal("the curse controller is not an opponent of itself but matched")
		}
	})

	t.Run("referent classifier accepts controls on every base", func(t *testing.T) {
		for _, ref := range []string{
			"Player.controlsElf_EQ0",
			"Opponent.controlsElf_EQ0",
			"You.controlsElf_EQ0",
		} {
			if !controlReferentPlayerSpecKnown(ref) {
				t.Errorf("controlReferentPlayerSpecKnown(%q) = false, want true", ref)
			}
		}
	})
}
