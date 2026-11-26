package kgen

import (
	"strings"
	"testing"
)

// animateShapesRemainingCarriers are the cards whose only Animate blocker is a
// shared-grant shape the earlier Animate passes left refused: a single static
// body that is not a Continuous/CombatDamageToughness/CantBlockBy shape
// (Carnage's MustAttack, Full Steam Ahead's MinMaxBlocker), a self
// Continuous body whose P/T is added rather than set (Iron Man Armor's "gets
// +1/+1 for each artifact you control") or that only adds a keyword (Restless
// Spire's conditional first strike), the CantBlockBy body with a blocker
// filter (Chomping Kavu's power-2 restriction), an AnimateAll Abilities$
// grant of a non-mana activated ability (Iron Fist's "{T}: deal damage"), and
// RemoveTypes$ naming a card type (Neurok Transmuter's "isn't an artifact").
var animateShapesRemainingCarriers = []string{
	"Carnage, Crimson Chaos",
	"Full Steam Ahead",
	"Iron Man Armor",
	"Restless Spire",
	"Chomping Kavu",
	"Iron Fist, Living Weapon",
	"Neurok Transmuter",
}

func TestAnimateShapesRemainingSlice(t *testing.T) {
	for _, name := range animateShapesRemainingCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// The lowered IR keeps the grant source-scoped: each static body is a static
// of the card that made the animation, switched on by the override record's
// GrantStatics bit (kop.Static.OvGate), and the AnimateAll non-mana grant
// names the maker's ability index like the Animate one does.
func TestAnimateShapesRemainingGrantShape(t *testing.T) {
	c := lowerNamedCard(t, "Carnage, Crimson Chaos")
	if !strings.Contains(overrideLIR(c), "grantstatics=1") {
		t.Errorf("Carnage's Animate lacks a granted static:\n%s", overrideLIR(c))
	}
	full := lowerNamedCard(t, "Full Steam Ahead")
	if !strings.Contains(overrideLIR(full), "grantstatics=1") {
		t.Errorf("Full Steam Ahead's AnimateAll lacks a granted static:\n%s", overrideLIR(full))
	}
	iron := lowerNamedCard(t, "Iron Man Armor")
	if !strings.Contains(overrideLIR(iron), "grantstatics=1") {
		t.Errorf("Iron Man Armor's Animate lacks a granted static:\n%s", overrideLIR(iron))
	}
	fist := lowerNamedCard(t, "Iron Fist, Living Weapon")
	if !strings.Contains(overrideLIR(fist), "grant=1") {
		t.Errorf("Iron Fist's AnimateAll lacks the granted ability:\n%s", overrideLIR(fist))
	}
}

// overrideLIR dumps the card's first override-carrying instruction: the grant
// fields (grantstatics=, grant=) live on the Override / OverrideAll note, so a
// match elsewhere on the card cannot satisfy an assertion.
func overrideLIR(c *lCard) string {
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == "Override" || in.Op == "OverrideAll" {
				return in.String()
			}
		}
	}
	return ""
}
