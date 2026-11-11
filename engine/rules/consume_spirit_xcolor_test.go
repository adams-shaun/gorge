package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// TestConsumeSpiritXColor pins the CR 107.3 restriction "Spend only black mana
// on X": the SpellAbility's XColor$ Black turns each announced X into a black
// pip, so the payer cannot buy the X with another colour. The printed cost is
// {X}{1}{B}, so X = 1 is {1}{B}{B} (generic one, two black) and X = 2 adds a
// third black pip. A reader that ignores XColor$ folds X into generic and
// would accept a red or colourless source for it.
func TestConsumeSpiritXColor(t *testing.T) {
	const src = "Name:Consume Spirit\n" +
		"ManaCost:X 1 B\n" +
		"Types:Sorcery\n" +
		"A:SP$ DealDamage | NumDmg$ X | XColor$ Black | ValidTgts$ Any | SubAbility$ DBHeal\n" +
		"SVar:DBHeal:DB$ GainLife | LifeAmount$ X\n" +
		"Oracle:x\n"
	c, diags := cards.ParseBytes("consume_spirit.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parse diags: %+v", diags)
	}
	f := c.Faces[0]
	base := pay.WithSpellAbilityExtras(f, pay.FaceCostOf(f).Cost)
	// Preconditions the assertions below depend on: one printed black pip and
	// one {X}; and the ability's XColor$ was read.
	if base.Colored[2] != 1 || base.X != 1 {
		t.Fatalf("printed cost = %+v, want one black pip and one X", base)
	}
	if base.XColors != 1<<2 {
		t.Fatalf("XColors = %#b, want the black mask (bit 2)", base.XColors)
	}
	for _, tc := range []struct {
		x       int32
		generic int32
		black   int32
	}{
		{x: 0, generic: 1, black: 1},
		{x: 1, generic: 1, black: 2},
		{x: 2, generic: 1, black: 3},
	} {
		got := base.WithX(tc.x)
		if got.X != 0 || got.XMin != 0 {
			t.Errorf("X = %d: announcement not consumed: %+v", tc.x, got)
		}
		if got.Generic != tc.generic || got.Colored[2] != tc.black {
			t.Errorf("X = %d: {generic %d, black %d}, want {generic %d, black %d}",
				tc.x, got.Generic, got.Colored[2], tc.generic, tc.black)
		}
	}
}
