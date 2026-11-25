package kgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReplDmgAskBodiesSlice: the printed DamageDone replacements whose body
// asks its controller a choice lower (lower_replinstead.go): Dralnu, Lich Lord
// (its controller sacrifices that many permanents of their choice) and
// Phyrexian Vindicator (the damage is prevented and it deals that much damage
// to a target its controller picks among any other creature, player,
// planeswalker or battlefield permanent). Each card lowers as
// ReplDamageInstead with its own body bit, and a variant off the shape still
// refuses by name.
func TestReplDmgAskBodiesSlice(t *testing.T) {
	const prev = kop.ReplDmgInsPrevent
	for _, c := range []struct{ card, want string }{
		{"Dralnu, Lich Lord", fmt.Sprintf("bits=%#x body=%#x ctr=%q", kop.ReplDmgSelf, kop.ReplDmgInsSac, "")},
		{"Phyrexian Vindicator", fmt.Sprintf("bits=%#x body=%#x ctr=%q", kop.ReplDmgSelf, prev|kop.ReplDmgInsVind, "")},
	} {
		lir, ref := lowerCarriers(t, []string{c.card})
		for _, r := range ref[c.card] {
			t.Errorf("%s: refused: %s", c.card, r)
		}
		if !strings.Contains(lir, "repl DamageInstead") || !strings.Contains(lir, c.want) {
			t.Errorf("%s: want a DamageInstead line with %s, got\n%s", c.card, c.want, lir)
		}
	}
}
