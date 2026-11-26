package kgen

import (
	"strings"
	"testing"
)

// TestRaiseCostAbilityLowers: S:Mode$ RaiseCost | Type$ Ability lowers to
// kop.StatRaiseAbility for the corpus shapes the kernel prices (T18c):
// Brutal Suppression's "activated abilities of nontoken Rebels cost an
// additional 'Sacrifice a land' to activate" (a Sac<1/Land> pick posed at the
// activation's cost stage), Oppressive Rays' "activated abilities of
// enchanted creature cost {3} more to activate" (a generic raise beside its
// CantAttackUnless/CantBlockUnless statics) and Thran Portal's "mana abilities
// of CARDNAME cost an additional 1 life to activate" (ValidSpell$
// Activated.ManaAbility with a self filter, the life a StatRaiseAbility
// StRaiseLife raise the mana window pays at each tap of the source;
// t18c-thran-portal-mana-ability-raise). Anointed Peacekeeper's
// Activated.!ManaAbility raise keeps its refusal.
func TestRaiseCostAbilityLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Brutal Suppression", "Oppressive Rays", "Thran Portal"})
	if len(cs) != 3 {
		t.Fatalf("lowered %d cards", len(cs))
	}
	lir := map[string]string{}
	for _, c := range cs {
		var sb strings.Builder
		dumpLIR(&sb, c)
		lir[c.Name] = sb.String()
	}
	for _, c := range cs {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s: %s", c.Name, r.Node, r.Reason)
		}
	}
	for name, want := range map[string]string{
		"Brutal Suppression": "static RaiseCost",
		"Oppressive Rays":    "static RaiseCost",
		"Thran Portal":       "static RaiseCost",
	} {
		if !strings.Contains(lir[name], want) {
			t.Errorf("%s: lowered IR lacks %q:\n%s", name, want, lir[name])
		}
	}
	for name, want := range map[string]string{
		"Brutal Suppression": "flags=[SacPick]",
		"Oppressive Rays":    "amount=3",
		"Thran Portal":       "flags=[ManaLife]",
	} {
		if !strings.Contains(lir[name], want) {
			t.Errorf("%s: lowered IR lacks %q:\n%s", name, want, lir[name])
		}
	}
	if !strings.Contains(lir["Brutal Suppression"], "F2={types=0x1}") {
		t.Errorf("Brutal Suppression: lowered IR lacks the land pick (F2):\n%s", lir["Brutal Suppression"])
	}
	if got := lir["Thran Portal"]; !strings.Contains(got, "amount=1") {
		t.Errorf("Thran Portal: want the one-life raise amount, got:\n%s", got)
	}
}
