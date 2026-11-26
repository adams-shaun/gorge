package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// xRaiseScript is a sorcery whose own RaiseCost Cost$ is the argument, beside
// a printed {X} and the SVar X the caller names.
func xRaiseScript(mana, cost, xvar string) string {
	return "Name:Probe X\nManaCost:" + mana + "\nTypes:Sorcery\n" +
		"S:Mode$ RaiseCost | ValidCard$ Card.Self | Activator$ You | Type$ Spell | Cost$ " + cost +
		" | EffectZone$ All | Description$ Probe.\n" +
		"A:SP$ Draw | Cost$ " + mana + " | NumCards$ X | SpellDescription$ Probe.\n" +
		"SVar:X:" + xvar + "\nOracle:Probe.\n"
}

// TestCostProgCastRouteRemainderRest: a card's own RaiseCost Cost$ of an
// announced-X pick (Aether Tide's Discard<X/Creature>) rides the spell's own
// additional cost field. The kernel's pick stage already pays DiscardX and
// SacX on CardDef.SpellCost / the AbSpell's cost (cost_discardfiltered.go,
// cost_sacx.go), so the only gap was lowering's gate. The part is announced
// only when the spell prints {X} and its SVar X reads the announced amount
// (Count$xPaid); Tectonic Split's Sac<X/Land> is "half your lands", a
// computed number the announce stage must not bind, and stays refused.
func TestCostProgCastRouteRemainderRest(t *testing.T) {
	all, bs := costProgCastBuild(t, []string{"Aether Tide", "Tectonic Split"})
	if len(all) != 2 || len(bs) != 2 {
		t.Fatalf("lowered %d cards, built %d, want two of each", len(all), len(bs))
	}
	if got := costProgCastRouteReason(all[0]); got != "" {
		t.Fatalf("Aether Tide: refusals %q, want none (the pick stage pays Discard<X>)", got)
	}
	sp := spellAbility(bs[0].def)
	if sp == nil {
		t.Fatal("Aether Tide built without a spell ability")
	}
	if sp.Cost.Flags&kop.CostDiscard == 0 || !sp.Cost.DiscardX {
		t.Fatalf("Aether Tide spell cost %+v, want the announced-X discard (CostDiscard, DiscardX)", sp.Cost)
	}
	// Tectonic Split's half-the-lands count lowers as kop.Cost.SacHalf
	// (TestRaiseCostComputedPickLowers)
	if got := costProgCastRouteReason(all[1]); got != "" {
		t.Errorf("Tectonic Split refusals %q, want none (the kernel counts the lands)", got)
	}

	// the same shape with a computed X must not lower as an announced part
	if got := buildRefusals(t, "Probe X", xRaiseScript("X U", "Discard<X/Creature/creature(s)>", "Count$Valid Land.YouCtrl")); got == "" {
		t.Error("a self Discard<X> raise whose X is a computed count lowered (the announce stage would bind it)")
	}
	// ... and one with no printed {X} has nothing to announce
	if got := buildRefusals(t, "Probe X", xRaiseScript("1 U", "Discard<X/Creature/creature(s)>", "Count$xPaid")); got == "" {
		t.Error("a self Discard<X> raise on a spell with no printed {X} lowered")
	}
	// the announced shape with a printed {X} lowers
	if got := buildRefusals(t, "Probe X", xRaiseScript("X U", "Discard<X/Creature/creature(s)>", "Count$xPaid")); got != "" {
		t.Errorf("announced Discard<X> raise refused: %q", got)
	}
}
