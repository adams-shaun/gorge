package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Rhystic Cave's Forge chain: AB$ ChooseColor | Cost$ T | InstantSpeed$ True |
// SubAbility$ DBMana, then DB$ Mana | Produced$ Chosen | UnlessCost$ 1 |
// UnlessPayer$ Player, then DB$ Cleanup | ClearChosenColor$ True. Gorge classes
// the chain a CR 605.1a mana ability and its off-stack frame asks every player
// whether to pay {1} before the mana is added (engine/rules
// mana_activation.go askManaUnless). The kernel's colour-asking mana arm
// carries the same ask as kop.Ability.ManaUnless.
const dbManaRhysticCave = "Name:Test Rhystic Cave\nManaCost:no cost\nTypes:Land\n" +
	"A:AB$ ChooseColor | Cost$ T | InstantSpeed$ True | SubAbility$ ProdStep | SpellDescription$ Probe.\n" +
	"SVar:ProdStep:DB$ Mana | Produced$ Chosen | UnlessCost$ 1 | UnlessPayer$ Player | SubAbility$ ClearStep\n" +
	"SVar:ClearStep:DB$ Cleanup | ClearChosenColor$ True\n"

// TestDbManaRhysticCaveLowers: the constant one-mana chosen-colour chain whose
// Mana node carries UnlessCost$ 1 | UnlessPayer$ Player is a CR 605.1a mana
// ability (gorge resolves it off the stack). It lowers as one colour-asking
// mana ability with the unless flag; any other unless shape, and any other
// amount, stays refused by the ask-colour arm.
func TestDbManaRhysticCaveLowers(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"the printed chain", dbManaRhysticCave},
		{"an explicit Amount$ 1", strings.Replace(dbManaRhysticCave, "Produced$ Chosen |", "Produced$ Chosen | Amount$ 1 |", 1)},
		{"no InstantSpeed$", strings.Replace(dbManaRhysticCave, " | InstantSpeed$ True", "", 1)},
	} {
		def, rs, _ := dbManaChooseBuild(t, "Test Rhystic Cave", c.src)
		if len(rs) != 0 {
			t.Errorf("%s: refused %v", c.name, rs)
			continue
		}
		var ab *kop.Ability
		n := 0
		for i := range def.Abilities {
			if def.Abilities[i].Kind == kop.AbMana {
				n++
				ab = &def.Abilities[i]
			}
		}
		if n != 1 || ab == nil {
			t.Errorf("%s: %d mana abilities, want 1", c.name, n)
			continue
		}
		if !ab.ManaAskColour || !ab.ManaUnless {
			t.Errorf("%s: ManaAskColour=%v ManaUnless=%v, want both", c.name, ab.ManaAskColour, ab.ManaUnless)
		}
		if ab.ManaColors != kop.ColAny || ab.ManaN != 1 || ab.ManaV != (kop.ValueRef{}) {
			t.Errorf("%s: colours %#x N %d V %+v, want one any-colour mana", c.name, ab.ManaColors, ab.ManaN, ab.ManaV)
		}
		if ab.Cost.Flags&kop.CostTap == 0 || ab.Cost.Mana != (kop.ManaCost{}) || len(ab.Prog) != 0 {
			t.Errorf("%s: cost %+v prog %v, want a bare {T} and no program", c.name, ab.Cost, ab.Prog)
		}
	}
	// any other unless shape, and any other amount, stays refused by the
	// colour-asking arm (a named refusal, not a silent drop)
	for _, c := range []struct{ name, src string }{
		{"another unless amount", strings.Replace(dbManaRhysticCave, "UnlessCost$ 1", "UnlessCost$ 2", 1)},
		{"another unless payer", strings.Replace(dbManaRhysticCave, "UnlessPayer$ Player", "UnlessPayer$ You", 1)},
		{"another amount", strings.Replace(dbManaRhysticCave, "Produced$ Chosen |", "Produced$ Chosen | Amount$ 2 |", 1)},
		{"a devotion amount", strings.Replace(dbManaRhysticCave, "Produced$ Chosen |", "Produced$ Chosen | Amount$ X |", 1) + "SVar:X:Count$Devotion.Chosen\n"},
		{"an unless beside a devotion amount", strings.Replace(dbManaNyxLotus, "Produced$ Chosen |", "Produced$ Chosen | UnlessCost$ 1 | UnlessPayer$ Player |", 1)},
	} {
		def, rs, _ := dbManaChooseBuild(t, "Test Rhystic Cave", c.src)
		var all []string
		for _, r := range rs {
			all = append(all, r.Reason)
		}
		n := 0
		for i := range def.Abilities {
			if def.Abilities[i].Kind == kop.AbMana && def.Abilities[i].ManaAskColour {
				n++
			}
		}
		if len(rs) == 0 || n != 0 || !strings.Contains(strings.Join(all, "; "), "ChooseColor") {
			t.Errorf("%s: %d asking abilities, refusals %q, want none and a ChooseColor refusal", c.name, n, all)
		}
	}
}
