package rules

import "testing"

// An {X} spell that reads the mana spent to cast it (Mockingbird's copy filter)
// is priced like any other {X} spell: its mana alone is what the planner asks, so
// a board that pays it only through a filter land is a play with a prefix, not a
// play the planner declines.
func TestPotentialPlanPricesAnXSpellThatReadsManaSpent(t *testing.T) {
	t.Parallel()
	spellSrc := corpusCardText(t, "m/mockingbird.txt")
	lands := []string{
		corpusCardText(t, "s/skycloud_expanse.txt"),
		corpusCardText(t, "p/plains.txt"),
	}
	e, _, spell := newFixtureDeck(t, 7, spellSrc, lands...)
	for _, l := range lands {
		putCreature(t, e, 0, l)
	}
	e.pending = nil
	e.priorityRound()
	seen := false
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Obj != spell || pp.Action.Kind != "cast" {
			continue
		}
		seen = true
		if pp.Reason != "" || pp.Plan == nil && len(pp.Script) == 0 {
			t.Fatalf("Mockingbird over a Skycloud Expanse and a Plains: reason %q detail %q plan %v script %v, want a priced play",
				pp.Reason, pp.Detail, pp.Plan != nil, pp.Script)
		}
	}
	if !seen {
		t.Fatal("the potential walk lists no cast of Mockingbird")
	}
}
