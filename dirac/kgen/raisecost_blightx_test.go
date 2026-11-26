package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// blightXScript is a sorcery whose own RaiseCost Cost$ is Blight<X>, beside
// the spell body's Announce$ X and the XMax$ and SVars the caller names.
func blightXScript(mana, announce, xmax, xvar, grto string) string {
	return "Name:Probe B\nManaCost:" + mana + "\nTypes:Sorcery\n" +
		"S:Mode$ RaiseCost | ValidCard$ Card.Self | Activator$ You | Type$ Spell | Cost$ Blight<X>" +
		" | EffectZone$ All | Description$ Probe.\n" +
		"A:SP$ Draw | NumCards$ X" + announce + xmax + " | SpellDescription$ Probe.\n" +
		"SVar:X:" + xvar + "\nSVar:GrTo:" + grto + "\nOracle:Probe.\n"
}

// TestRaiseCostSoulImmolationLowers: a card's own RaiseCost Cost$ Blight<X>
// ("as an additional cost to cast this spell, blight X. X can't be greater
// than the greatest toughness among creatures you control", Soul Immolation)
// lowers as the spell's own additional cost, CostBlight with Cost.N 0: the
// announced X is the counter count, and the kernel bounds the X ask by the
// greatest toughness among the creatures the payer controls (gorge xAsk's
// blightX cap, `XMax$ GrTo`).
func TestRaiseCostSoulImmolationLowers(t *testing.T) {
	all, bs := costProgCastBuild(t, []string{"Soul Immolation"})
	if len(all) != 1 || len(bs) != 1 {
		t.Fatalf("lowered %d cards, built %d, want one of each", len(all), len(bs))
	}
	if got := costProgCastRouteReason(all[0]); got != "" {
		t.Fatalf("Soul Immolation refusals %q, want none", got)
	}
	sp := spellAbility(bs[0].def) // a sorcery's additional cost rides on its AbSpell
	if sp == nil {
		t.Fatal("Soul Immolation built without a spell ability")
	}
	c := sp.Cost
	if c.Flags&kop.CostBlight == 0 || c.N != 0 || c.Filter == 0 {
		t.Fatalf("Soul Immolation additional cost %+v, want CostBlight with N 0 and a creature filter", c)
	}

	const grTo = "Count$Valid Creature.YouCtrl$GreatestCardToughness"
	const ann, xmax = " | Announce$ X", " | XMax$ GrTo"
	if got := buildRefusals(t, "Probe B", blightXScript("3 R", ann, xmax, "Count$xPaid", grTo)); got != "" {
		t.Errorf("announced Blight<X> raise on a probe spell refused: %q", got)
	}
	for name, src := range map[string]string{
		"a computed X":          blightXScript("3 R", ann, xmax, "Count$Valid Land.YouCtrl", grTo),
		"a printed {X}":         blightXScript("X R", ann, xmax, "Count$xPaid", grTo),
		"no XMax$":              blightXScript("3 R", ann, "", "Count$xPaid", grTo),
		"no Announce$":          blightXScript("3 R", "", xmax, "Count$xPaid", grTo),
		"another bound (power)": blightXScript("3 R", ann, xmax, "Count$xPaid", "Count$Valid Creature.YouCtrl$GreatestPower"),
		"an opponent's bound":   blightXScript("3 R", ann, xmax, "Count$xPaid", "Count$Valid Creature.OppCtrl$GreatestCardToughness"),
	} {
		if got := buildRefusals(t, "Probe B", src); got == "" {
			t.Errorf("Blight<X> raise with %s lowered", name)
		}
	}
}
