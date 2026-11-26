package kgen

import (
	"strings"
	"testing"
)

// TestRaiseCostAbilityLifeRefused: a Type$ Ability raise whose Cost$ is a life
// part lowers to nothing. The kernel's StatRaiseAbility has no life path (the
// two corpus cards with one, Thran Portal and its twin, price it on their own
// mana abilities, a mana-window price; ticket
// t18c-thran-portal-mana-ability-raise), so a ValidSpell$-less shape must not
// slip through to a static the kernel would silently drop.
func TestRaiseCostAbilityLifeRefused(t *testing.T) {
	const src = "Name:Probe Pain\nManaCost:2\nTypes:Artifact\n" +
		"S:Mode$ RaiseCost | Type$ Ability | ValidCard$ Creature.YouCtrl | Cost$ PayLife<1> | AffectedZone$ Battlefield | Description$ probe\n" +
		"Oracle:Activated abilities of creatures you control cost an additional 1 life to activate.\n"
	c := lowerScriptCard(t, "Probe Pain", src)
	if len(c.Refusals) == 0 {
		t.Fatalf("a PayLife<1> ability raise lowered with no refusal")
	}
	var got []string
	for _, r := range c.Refusals {
		got = append(got, r.Reason)
	}
	if !strings.Contains(strings.Join(got, "\n"), "a life raise is priced only in the mana window") {
		t.Errorf("refusals %q lack the mana-window life reason", got)
	}
}
