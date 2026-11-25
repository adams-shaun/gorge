package kgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReplDmgShapesSlice: the printed DamageDone replacements that put the
// damage to another use lower (lower_replinstead.go): Angel of Suffering,
// Gloom Surgeon, Force Bubble, Panther Habit, Soul-Scar Mage, Swans of Bryn
// Argoll, Szadek and Undead Alchemist as ReplDamageInstead (the recipient in
// bits, the body in body), Ghosts of the Innocent as a halving
// ReplDamageMinus. Each card's dump line names its own shape, and a variant
// off the shape still refuses by name.
func TestReplDmgShapesSlice(t *testing.T) {
	const prev = kop.ReplDmgInsPrevent
	ins := func(bits, body int, ctr string) string {
		return fmt.Sprintf("bits=%#x body=%#x ctr=%q", bits, body, ctr)
	}
	for _, c := range []struct{ card, want string }{
		{"Angel of Suffering", ins(kop.ReplDmgYou, prev|kop.ReplDmgInsMill|kop.ReplDmgInsDouble, "")},
		{"Gloom Surgeon", ins(kop.ReplDmgSelf|kop.ReplDmgCombat, prev|kop.ReplDmgInsExileTop, "")},
		{"Force Bubble", ins(kop.ReplDmgYou, kop.ReplDmgInsCtrHost, "DEPLETION")},
		{"Panther Habit", ins(0, prev|kop.ReplDmgInsCtrEquipped, "P1P1")},
		{"Soul-Scar Mage", ins(kop.ReplDmgNonCombat, kop.ReplDmgInsCtrTarget, "M1M1")},
		{"Swans of Bryn Argoll", ins(kop.ReplDmgSelf, prev|kop.ReplDmgInsDrawSrc, "")},
		{"Szadek, Lord of Secrets", ins(kop.ReplDmgPlayer|kop.ReplDmgCombat, kop.ReplDmgInsCtrHost|kop.ReplDmgInsMill, "P1P1")},
		{"Undead Alchemist", ins(kop.ReplDmgPlayer|kop.ReplDmgCombat, kop.ReplDmgInsMill, "")},
	} {
		lir, ref := lowerCarriers(t, []string{c.card})
		for _, r := range ref[c.card] {
			t.Errorf("%s: refused: %s", c.card, r)
		}
		if !strings.Contains(lir, "repl DamageInstead") || !strings.Contains(lir, c.want) {
			t.Errorf("%s: want a DamageInstead line with %s, got\n%s", c.card, c.want, lir)
		}
	}

	lir, ref := lowerCarriers(t, []string{"Ghosts of the Innocent"})
	for _, r := range ref["Ghosts of the Innocent"] {
		t.Errorf("Ghosts of the Innocent: refused: %s", r)
	}
	if want := fmt.Sprintf("bits=%#x", kop.ReplDmgPlayer|kop.ReplDmgPerm|kop.ReplDmgHalfDown); !strings.Contains(lir, "repl DamageMinus") || !strings.Contains(lir, want) {
		t.Errorf("Ghosts of the Innocent: want a halving DamageMinus line with %s, got\n%s", want, lir)
	}
}
