package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// preventDamageCarriers are the paper cards the api:PreventDamage slice names:
// the PreventionSubAbility rider forms and the divided-as-you-choose forms.
var preventDamageCarriers = []string{
	"Acolyte's Reward", "Angel of Salvation", "Kitsune Palliator", "Samite Censer-Bearer",
	"Test of Faith", "Vengeful Archon", "Wojek Apothecary", "Embolden", "Fylgja",
	"Pollen Remedy", "Remedy", "Sacred Boon", "Scars of the Veteran", "Serra's Hymn", "Temper",
}

func TestPreventDamageFormsLower(t *testing.T) {
	for _, name := range preventDamageCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// TestPreventDamageFormShapes: each named card lowers to the instruction shape
// its form needs, so a card that merely stopped refusing cannot pass the
// lowering check.
func TestPreventDamageFormShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		flag int
	}{
		{"Test of Faith", int(kop.PrevNextRiderCounter)},
		{"Temper", int(kop.PrevNextRiderCounter)},
		{"Sacred Boon", int(kop.PrevNextRiderDelayedCounter)},
		{"Scars of the Veteran", int(kop.PrevNextRiderDelayedCounter)},
		{"Acolyte's Reward", int(kop.PrevNextRiderDamage)},
		{"Vengeful Archon", int(kop.PrevNextRiderDamage)},
		{"Angel of Salvation", int(kop.PrevNextDivided)},
		{"Embolden", int(kop.PrevNextDivided)},
		{"Pollen Remedy", int(kop.PrevNextDivided)},
		{"Remedy", int(kop.PrevNextDivided)},
		{"Serra's Hymn", int(kop.PrevNextDivided)},
		{"Kitsune Palliator", int(kop.PrevNextMultiObj)},
		{"Samite Censer-Bearer", int(kop.PrevNextMultiObj)},
		{"Wojek Apothecary", int(kop.PrevNextRadiance)},
	} {
		c := lowerNamedCard(t, tc.name)
		found := false
		for _, ab := range c.Abilities {
			for _, in := range ab.Prog {
				if in.Op == "PreventNext" && in.C&tc.flag != 0 {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s lowers without the %#x PreventNext flag", tc.name, tc.flag)
		}
		if tc.flag == int(kop.PrevNextDivided) {
			dived := false
			for _, ab := range c.Abilities {
				for _, tg := range ab.Targets {
					dived = dived || tg.Divided != 0
				}
			}
			if !dived {
				t.Errorf("%s lowers without a divided target slot", tc.name)
			}
		}
	}
	kit := lowerNamedCard(t, "Kitsune Palliator")
	playerHalf := false
	for _, ab := range kit.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "PreventNext" && in.C&int(kop.PrevNextMultiPlayer) != 0 {
				playerHalf = true
			}
		}
	}
	if !playerHalf {
		t.Error("Kitsune Palliator lowers without the each-player shield")
	}
	c := lowerNamedCard(t, "Fylgja")
	found := false
	for _, ab := range c.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "PreventNext" && in.Obj.Kind == rEquipped {
				found = true
			}
		}
	}
	if !found {
		t.Error("Fylgja lowers without a PreventNext on the enchanted creature")
	}
}
