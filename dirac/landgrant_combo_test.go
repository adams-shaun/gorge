package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A {T}: Add {G} or {W} land (Kyoshi Village, Produced$ Combo) is a plain
// {T} source: blgPlain accepts it, and rejects a simultaneous two-colour one.
func TestBlgPlainCombo(t *testing.T) {
	mk := func(flags kop.AbilityFlags) *kop.CardDef {
		return &kop.CardDef{Abilities: []kop.Ability{{
			Kind: kop.AbMana, AFlags: flags, ManaN: 1, ManaColors: kop.ColG | kop.ColW,
			Cost: kop.Cost{Flags: kop.CostTap},
		}}}
	}
	pc := &pcard{manaAb: 0}
	if !blgPlain(mk(kop.AfManaChoice), pc) {
		t.Error("a Combo {T} land is not plain-shaped")
	}
	if blgPlain(mk(0), pc) {
		t.Error("a simultaneous two-colour ability is plain-shaped")
	}
}
