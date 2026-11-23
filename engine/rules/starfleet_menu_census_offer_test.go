package rules

import (
	"testing"
)

// TestStarfleetMenuOfferFallsBackToCensusScript pins the interactive priority
// menu's census-gap arm (legal_walk_autopay.go's autoPayCensus): at a live
// priority ask (hyp nil) the menu offers Academy Ruins' {1}{U}, {T}
// graveyard-recall ability although the floating pool holds no {U} and the
// only {U} on the board is Starfleet Academy's Federation ability ({5},{T}:
// Add X of one chosen colour, X = 1 beside one Grizzly Bear) — the shape the
// planner's source census (paymentPlanManaUnits) misses, so
// planAbilityPayment answers "insufficient" and the canonical pricing proves
// the activation payable through its census relaxation and prefix script
// (CR 601.2g, 605.3b; TestStarfleetFundedActivationPriced). The menu must
// match that pricing: a seat that reads the posed decision never saw the
// option, so the Ruins card was dead in the seat's hands.
//
// The same ask also pins the pricing agreement around the arm's own offer:
// the arm marks its option (decision.Option.CensusOffer), so the canonical
// pricing's prefix search — which reads the posed decision through its
// search clones — still prices the activation payable with its script after
// the menu offered it (sr.offered skips the arm's script-backed offer, as
// the kernel's own listing probe excludes the arm's listings,
// actScriptPays' availNoActScript).
func TestStarfleetMenuOfferFallsBackToCensusScript(t *testing.T) {
	t.Parallel()

	e, ruins := sfuRuinsFixture(t, true, true)

	// Preconditions: the canonical pricing prices the activation (through its
	// prefix script), while the planner arm the menu arm falls back from
	// proves nothing — so the option below can only come from the census arm.
	got, script := sfuRuinsPlan(t, e, ruins)
	if got.Reason != "" || script == 0 {
		t.Fatalf("precondition: canonical pricing does not price the {1}{U} activation: reason %q script %d", got.Reason, script)
	}
	if out := e.planAbilityPayment(0, ruins, 1); out.Reason != "insufficient" {
		t.Fatalf("precondition: planAbilityPayment answers %q (want insufficient, detail %q)", out.Reason, out.Detail)
	}

	opts := e.legalActions(0)
	found := false
	for _, o := range opts {
		if o.Kind == "ability" && o.Obj == ruins && o.Ability == 1 {
			if !o.CensusOffer {
				t.Fatalf("the %v option came from the plain walk, not the census arm", o.Kind)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("priority menu offers no ability option for the Federation-funded {1}{U} activation of ruins obj")
	}

	// The arm's offer must not unprice the activation the canonical pricing
	// proved: the search reads the posed decision through its clones, and an
	// unmarked offer there is taken for "the plain walk lists it" — an empty
	// prefix that prices nothing (dirac/legal.go availNoActScript).
	after, afterScript := sfuRuinsPlan(t, e, ruins)
	if after.Reason != "" || afterScript == 0 {
		t.Fatalf("after the menu offered it, canonical pricing no longer prices the {1}{U} activation: reason %q script %d", after.Reason, afterScript)
	}
}
