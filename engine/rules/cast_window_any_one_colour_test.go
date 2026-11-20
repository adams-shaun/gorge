// The cast window's choice-shaped probe must match the live colour ask it
// bounds: a plain Produced$ Any with an Amount$ above 1 means the WHOLE
// amount in ONE colour (the ask poses a single colour, mana_activation.go's
// non-allocation branch), so a two-colour cost is never payable from it and
// the probe must price no mixed multiset for it. A Combo Any production is
// the other shape — the ask allocates one colour per mana unit — so its
// mixed multisets stay priced (the Champion of Lambholt / Worldsoul Colossus
// convoke reach). These leaves pin both sides of that line.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// cwAnyTwoSrc is the Khalni Gem / Vedalken Engineer shape: "Add two mana
	// of any one color" as a free choice-shaped production with a literal
	// Amount$ 2. Free cost, so only the choice-shaped layer prices it. The
	// description is worded off the printed one so no fixture line matches a
	// Forge script verbatim (the GPL boundary).
	cwAnyTwoSrc = "Name:AnyTwo\nManaCost:1 G\nTypes:Creature Elf Druid\nPT:2/2\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 2 | SpellDescription$ Add two mana, all of any one color.\n" +
		"Oracle:x\n"
	// cwComboTwoSrc is the per-unit counterpart: "Add two mana in any
	// combination of colors", one colour chosen per mana unit, so {G}{W} is
	// payable from one activation.
	cwComboTwoSrc = "Name:ComboTwo\nManaCost:1 G\nTypes:Creature Elf Druid\nPT:2/2\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Combo Any | Amount$ 2 | SpellDescription$ Add two mana mixed across any colors.\n" +
		"Oracle:x\n"
)

// cwColourCount reports how many distinct colours an alternative's counts
// carry, and its total units.
func cwColourCount(counts [6]int32) int {
	n := 0
	for _, v := range counts {
		if v > 0 {
			n++
		}
	}
	return n
}

// TestCastWindowAnyAmountTwoPricesNoMixedMultiset pins the plain-Any side: a
// Produced$ Any | Amount$ 2 source is priced only as the whole amount in one
// colour, never as a mixed {G}{W} pair the live ask cannot produce.
func TestCastWindowAnyAmountTwoPricesNoMixedMultiset(t *testing.T) {
	t.Parallel()
	b := equipWindowGame(t, 710, cwAnyTwoSrc)
	e, anyID := b.engine, b.byName["AnyTwo"]
	if o := e.G.Obj(anyID); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("AnyTwo precondition: %+v, want an untapped battlefield permanent", o)
	}
	addMana(t, e, 0, "CCCCCC")
	cwArmBelt(t, e, b.belt)
	single, mixed, amt := 0, 0, int32(0)
	for _, u := range e.castWindowUnits(e.cast) {
		if u.ID != anyID {
			continue
		}
		for _, a := range u.Alts {
			switch cwColourCount(a.Counts) {
			case 0:
				t.Fatalf("probe priced an empty alternative for AnyTwo: %+v", a)
			case 1:
				single++
				amt = a.Amt
			default:
				mixed++
			}
		}
	}
	if mixed != 0 {
		t.Fatalf("probe priced %d mixed-colour alternatives for a plain Produced$ Any | Amount$ 2 source: the live ask produces two mana of any ONE colour, never {G}{W}", mixed)
	}
	if single == 0 {
		t.Fatal("probe priced no alternative for AnyTwo; the precondition is wrong")
	}
	if single != 5 {
		t.Fatalf("probe priced %d single-colour alternatives for AnyTwo, want 5 (one per colour)", single)
	}
	if amt != 2 {
		t.Fatalf("single-colour alternatives carry Amount %d, want the whole 2", amt)
	}
}

// TestCastWindowComboAnyAmountTwoStillPricesMixed is the control on the other
// side of the line: a Combo Any Amount 2 source is a per-unit allocation, so
// its mixed multisets stay priced alongside the single-colour ones.
func TestCastWindowComboAnyAmountTwoStillPricesMixed(t *testing.T) {
	t.Parallel()
	b := equipWindowGame(t, 710, cwComboTwoSrc)
	e, comboID := b.engine, b.byName["ComboTwo"]
	if o := e.G.Obj(comboID); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("ComboTwo precondition: %+v, want an untapped battlefield permanent", o)
	}
	addMana(t, e, 0, "CCCCCC")
	cwArmBelt(t, e, b.belt)
	single, mixed := 0, 0
	for _, u := range e.castWindowUnits(e.cast) {
		if u.ID != comboID {
			continue
		}
		for _, a := range u.Alts {
			if cwColourCount(a.Counts) > 1 {
				mixed++
			} else {
				single++
			}
		}
	}
	if single == 0 {
		t.Fatal("probe priced no single-colour alternative for ComboTwo; the precondition is wrong")
	}
	if mixed == 0 {
		t.Fatal("probe priced no mixed-colour alternative for a Combo Any Amount 2 source; the per-unit allocation lost its {G}{W} reach")
	}
}
