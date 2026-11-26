package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestKickerTwoCostLowers: the colon-form Kicker (Forge's K:Kicker:<a>:<b>,
// the printed "Kicker {a} and/or {b}", CR 702.33b) lowers into CardDef.Kicker
// (part one) and CardDef.Kicker2 (part two) instead of refusing. The five
// fully-lowered carriers at the pin lower with no refusals; Urborg Lhurgoyf
// loses its kicker-cost refusal and keeps exactly its separate ETB
// replacement api Mill one (asserted by reason, so this test cannot silently
// absorb it).
func TestKickerTwoCostLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{
		"Stormscape Battlemage", "Wastescape Battlemage", "Thornscape Battlemage",
		"Stronghold Arena", "Archangel of Wrath",
	} {
		t.Run(name, func(t *testing.T) {
			lc := lowerTwoAltCard(t, reg, name)
			if len(lc.Refusals) != 0 {
				t.Fatalf("%s refusals = %v, want none", name, lc.Refusals)
			}
			if lc.Kicker == nil || lc.Kicker2 == nil {
				t.Fatalf("%s lowered without Kicker/Kicker2 (Kicker=%v Kicker2=%v)", name, lc.Kicker, lc.Kicker2)
			}
		})
	}
	t.Run("Urborg Lhurgoyf", func(t *testing.T) {
		lc := lowerTwoAltCard(t, reg, "Urborg Lhurgoyf")
		if lc.Kicker == nil || lc.Kicker2 == nil {
			t.Fatalf("Urborg Lhurgoyf lowered without Kicker/Kicker2")
		}
		if len(lc.Refusals) != 1 || !strings.Contains(lc.Refusals[0].Reason, "ETB replacement api Mill") {
			t.Fatalf("Urborg Lhurgoyf refusals = %v, want exactly the ETB replacement api Mill one", lc.Refusals)
		}
	})
}

func lowerTwoAltCard(t *testing.T, reg *cards.Registry, name string) *lCard {
	t.Helper()
	return lowerMultikickerCard(t, reg, name)
}

// TestKicker2BesideMultikickerRefuses: the two-alternative Kicker's part two
// keeps one kicked family per cast (CR 702.33b beside CR 702.43; the kernel's
// count stores and kicked offers are one family's). Kicker2 is only ever set
// beside Kicker (lower.go's Kicker head), and refuseMultikickerCombos refuses
// Multikicker beside any additional cost -- Kicker among them -- so a card
// printing both refuses, in either keyword order (design decision 2). No
// corpus card has both at the pin. The Multikicker-only control proves the
// refusal is the combination's, not the probe card's.
func TestKicker2BesideMultikickerRefuses(t *testing.T) {
	for _, order := range []struct{ name, src string }{
		{"kicker first", `Name:Probe Twoalt
ManaCost:2 G
Types:Creature Elf Wizard
PT:2/2
K:Kicker:R:W
K:Multikicker:2
`},
		{"multikicker first", `Name:Probe Twoalt
ManaCost:2 G
Types:Creature Elf Wizard
PT:2/2
K:Multikicker:2
K:Kicker:R:W
`},
	} {
		t.Run(order.name, func(t *testing.T) {
			lc := lowerScriptCard(t, "Probe Twoalt", order.src)
			if len(lc.Refusals) == 0 {
				t.Fatalf("a Kicker2 + Multikicker card lowered (Kicker=%v Kicker2=%v Multikicker=%v), want a refusal", lc.Kicker, lc.Kicker2, lc.Multikicker)
			}
			if !strings.Contains(lc.Refusals[0].Reason, "Multikicker beside") {
				t.Fatalf("refusal %q, want the Multikicker-combination one", lc.Refusals[0].Reason)
			}
		})
	}
	t.Run("multikicker alone lowers", func(t *testing.T) {
		lc := lowerScriptCard(t, "Probe Twoalt", `Name:Probe Twoalt
ManaCost:2 G
Types:Creature Elf Wizard
PT:2/2
K:Multikicker:2
`)
		if len(lc.Refusals) != 0 {
			t.Fatalf("the Multikicker-only control refused: %v", lc.Refusals)
		}
	})
}
