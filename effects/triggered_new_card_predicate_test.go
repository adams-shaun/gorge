package effects

import "testing"

// TestTriggeredNewCardPredicateResolvesAndFailsClosed (exg1): Forge's bare
// TriggeredNewCard / TriggeredCard property -- the "you may exile it" cost
// idiom's spec (`Cost$ ExileAnyGrave<1/Card.TriggeredNewCard>`, Cavalier of
// Thorns and 17 more corpus carriers) -- matches exactly the card the
// triggering event captured, through SpecContext.TriggerContext, and fails
// closed (matches nothing) wherever no trigger context is bound, so the
// ordinary activated path never invents a referent. The census classifier
// must agree with the matcher.
func TestTriggeredNewCardPredicateResolvesAndFailsClosed(t *testing.T) {
	g, ids := board(t)
	card := ids["myBear"]
	decoy := ids["myLand"]
	sc := SpecContext{TriggerContext: TriggerContext{TriggerCard: card}}
	for _, spec := range []string{"Card.TriggeredNewCard", "Card.TriggeredCard"} {
		if !MatchesSpecCtx(g, spec, card, sc) {
			t.Fatalf("%s did not match the triggering card", spec)
		}
		if MatchesSpecCtx(g, spec, decoy, sc) {
			t.Fatalf("%s matched a non-triggering decoy", spec)
		}
		// The fail-closed default: MatchesSpecFrom (the cost sites' source-only
		// form) carries no trigger context, so the spec matches nothing.
		if MatchesSpecFrom(g, spec, card, 0, card) {
			t.Fatalf("%s matched without a bound trigger context", spec)
		}
		if unknown := UnknownPredicates(spec); len(unknown) != 0 {
			t.Fatalf("%s: the census classifier disagrees with the matcher: %v", spec, unknown)
		}
	}
}
