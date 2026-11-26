package kgen

import (
	"strings"
	"testing"
)

// TestRaiseCostAbilityLowers: S:Mode$ RaiseCost | Type$ Ability lowers to
// kop.StatRaiseAbility for the two corpus shapes the kernel prices (T18c):
// Brutal Suppression's "activated abilities of nontoken Rebels cost an
// additional 'Sacrifice a land' to activate" (a Sac<1/Land> pick posed at the
// activation's cost stage) and Oppressive Rays' "activated abilities of
// enchanted creature cost {3} more to activate" (a generic raise beside its
// CantAttackUnless/CantBlockUnless statics). Thran Portal's raise is its own
// mana abilities' life (ValidSpell$ Activated.ManaAbility): the kernel's mana
// window prices no ability raise, so the static keeps its named refusal
// (t18c-raisecost-ability follow-up).
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
		if c.Name == "Thran Portal" {
			continue
		}
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s: %s", c.Name, r.Node, r.Reason)
		}
	}
	for name, want := range map[string]string{
		"Brutal Suppression": "static RaiseAbility",
		"Oppressive Rays":    "static RaiseAbility",
	} {
		if !strings.Contains(lir[name], want) {
			t.Errorf("%s: lowered IR lacks %q:\n%s", name, want, lir[name])
		}
	}
	for name, want := range map[string]string{
		"Brutal Suppression": "RaiseAbility F={",
		"Oppressive Rays":    "amount=3",
	} {
		if !strings.Contains(lir[name], want) {
			t.Errorf("%s: lowered IR lacks %q:\n%s", name, want, lir[name])
		}
	}
	if !strings.Contains(lir["Brutal Suppression"], "F2={types=0x1}") {
		t.Errorf("Brutal Suppression: lowered IR lacks the land pick (F2):\n%s", lir["Brutal Suppression"])
	}
	if got := lir["Thran Portal"]; !strings.Contains(got, `RaiseCost Type$ Ability with ValidSpell$ "Activated.ManaAbility"`) {
		t.Errorf("Thran Portal: want the mana-window refusal, got:\n%s", got)
	}
}
