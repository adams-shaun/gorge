package rules

import "testing"

// A spell that reads the mana spent to cast it is priced from its composed cost
// like any other: the cast planner declines the shape, but the price is the
// printed one, so a board that pays it only through a filter land is a play with
// a prefix, not a play the potential planner leaves unpriced.
func TestPotentialPlanPricesASpellThatReadsManaSpentThroughAFilter(t *testing.T) {
	t.Parallel()
	spellSrc := `Name:Tally Scrivener
ManaCost:W U
Types:Instant
A:SP$ Draw | NumCards$ Tally | SpellDescription$ Draw a card for each mana spent to cast this spell.
SVar:Tally:Count$CastTotalManaSpent
`
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
			t.Fatalf("a spell reading its mana spent over a Skycloud Expanse and a Plains: reason %q detail %q plan %v script %v, want a priced play",
				pp.Reason, pp.Detail, pp.Plan != nil, pp.Script)
		}
	}
	if !seen {
		t.Fatal("the potential walk lists no cast of the fixture spell")
	}
}
