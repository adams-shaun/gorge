package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLowerNmanaNotFixedSet: the cards whose second restricted mana ability's
// spend restriction names activated abilities lower (Sunken Citadel's chosen
// {C}{C} for abilities of land sources beside its chosen {C}; Steelswarm
// Operator's artifact-activation {U}{U} beside its artifact-spell {U}), and
// the out-of-shape neighbours stay refused by name. The kernel plans the
// restricted pair or the two disjoint restricted abilities
// (mana_restrictedsource.go prepRestrSrc, restrViewW); the older classifier
// refused every activation-only restriction.
func TestLowerNmanaNotFixedSet(t *testing.T) {
	const citadel, operator = "Sunken Citadel", "Steelswarm Operator"
	lw := lowerNamedPool(t, []string{citadel, operator})
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, c := range lw.cards {
		if c.Name != citadel && c.Name != operator {
			continue
		}
		seen[c.Name]++
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused at lowering: %v", c.Name, c.Refusals)
		}
	}
	for _, b := range bs {
		n := b.def.Name
		if n != citadel && n != operator {
			continue
		}
		if len(b.refusals) != 0 {
			t.Errorf("%s refused at build: %v", n, b.refusals)
		}
	}
	for _, n := range []string{citadel, operator} {
		if seen[n] != 1 {
			t.Errorf("%s lowered %d times, want 1", n, seen[n])
		}
	}

	// the Operator's two abilities survive as two restricted records: the
	// first spell-only, the second activation-only (the kernel's
	// ManaRestrictAb + restrVarOf kind)
	var op *kop.CardDef
	for i := range bs {
		if bs[i].def.Name == operator {
			op = &bs[i].def
		}
	}
	if op == nil {
		t.Fatal("Steelswarm Operator has no lowered card def: the case reads nothing")
	}
	spell, act := 0, 0
	for i := range op.Abilities {
		a := &op.Abilities[i]
		if a.Kind != kop.AbMana {
			continue
		}
		if a.ManaRestrict != 0 && a.ManaRestrictAb == 0 {
			spell++
		}
		if a.ManaRestrict != 0 && a.ManaRestrictAb != 0 {
			act++
		}
	}
	if spell != 1 || act != 1 {
		t.Errorf("Steelswarm Operator abilities: %d spell-only, %d activation-only restricted, want 1 and 1", spell, act)
	}

	// shapes: a chosen activation-only larger ability beside its chosen first
	citadelFirst := comboAb("C", 1, false, false)
	citadelFirst.Mana.Colors = "Chosen"
	citadelSecond := comboAb("C", 2, false, false)
	citadelSecond.Mana.Colors = "Chosen"
	citadelSecond.Mana.RestrictAb = &lFilter{}
	if !manaGapModelled(citadelFirst, citadelSecond) {
		t.Error("chosen {C}{C} for land-source abilities beside chosen {C}: refused")
	}

	// a spell-only first ability beside an activation-only larger one of the
	// same colour (the Operator's shape)
	opFirst := comboAb("U", 1, false, true)
	opSecond := comboAb("U", 2, false, false)
	opSecond.Mana.RestrictAb = &lFilter{}
	first := comboAb("U", 1, false, false)
	if !manaGapModelled(first, opSecond) {
		t.Error("an activation-only {U}{U} beside a plain {U}: refused")
	}
	if !restrictedFirstPair(opFirst, opSecond) {
		t.Error("restrictedFirstPair refused the Operator's shape")
	}

	// out-of-shape variants stay refused, and the activation-only one names
	// itself
	actFive := comboAb("U", 5, false, false)
	actFive.Mana.RestrictAb = &lFilter{}
	actOther := comboAb("G", 2, false, false)
	actOther.Mana.RestrictAb = &lFilter{}
	bothHalves := comboAb("U", 2, false, true)
	bothHalves.Mana.RestrictAb = &lFilter{}
	bothFirst := comboAb("U", 1, false, true)
	bothFirst.Mana.RestrictAb = &lFilter{}
	for name, c := range map[string]struct{ first, a *lAbility }{
		"activation amount 5":        {first, actFive},
		"activation other colour":    {first, actOther},
		"spell and activated halves": {first, bothHalves},
		"both halves on the first":   {bothFirst, opSecond},
		"activation amount 1":        {first, func() *lAbility { a := comboAb("U", 1, false, false); a.Mana.RestrictAb = &lFilter{}; return a }()},
		"different first colour":     {comboAb("G", 1, false, false), opSecond},
	} {
		if manaGapModelled(c.first, c.a) {
			t.Errorf("%s: accepted", name)
			continue
		}
		if name == "activation amount 5" {
			if why := manaGapWhy(c.first, c.a); !strings.Contains(why, "activation-only restricted ability") {
				t.Errorf("amount 5 refusal %q does not name the activation-only restriction", why)
			}
		}
	}
}
