package rules

// A battlefield static whose RaiseCost$ does not GRANT the card must not
// report a raise for it (d09db26e34): mayPlayRaiseCost walks every
// mayPlayStaticBenefits battlefield static, so before that fix Festival of
// Embers -- Affected$ Instant.YouCtrl,Sorcery.YouCtrl, RaiseCost$
// PayLife<1> -- leaked its 1-life raise onto Alien Symbiosis, an Aura in
// hand that the static covers at no zone and no card type. The leaked
// hasRaise offered the hand card a may-play surcharge the kernel never
// prices (the TestMayraiseLockstep divergence this pins), and would
// withhold a LAND outright.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMayPlayRaiseCostIgnoresNonGrantingStatic: a present, priced raise
// from a static that does not apply to (or grant) the card contributes no
// hasRaise and opens no mayplay offer for the hand card.
func TestMayPlayRaiseCostIgnoresNonGrantingStatic(t *testing.T) {
	t.Parallel()
	bear := card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	alien := corpusAlternativeCard(t, "Alien Symbiosis")
	fest := corpusAlternativeCard(t, "Festival of Embers")
	e := handEngine(t, bear, alien, fest)
	hand := e.G.Zone(state.ZHand, 0)
	bearID, alienID, festID := hand[0], hand[1], hand[2]
	placeOnBattlefield(t, e, bearID)
	placeOnBattlefield(t, e, festID)

	// Precondition: Festival's raise is present and priced for the hand
	// Alien, and the static neither applies to nor grants it -- the exact
	// shape the unguarded walk took. MayPlayRaiseCost's battlefield walk
	// runs every mayPlayStaticBenefits static through mayPlayStatic, so the
	// same read is made here; compared values differ (has/priced true,
	// applies/grants false), so the walk's guard has something to skip on.
	// (The failing AffectedZone$ gate zeroes the raise composition while
	// keeping has/priced, so only those two booleans are observable here.)
	sawFestRaise := false
	for _, sv := range e.activeStatics("Continuous") {
		if !mayPlayStaticBenefits(0, &sv) {
			continue
		}
		applies, grants, _, _, has, priced := e.mayPlayStatic(sv.Params, alienID, mayPlayStaticYou(0, &sv), sv.Source)
		if has && priced && !applies && !grants {
			sawFestRaise = true
		}
	}
	if !sawFestRaise {
		t.Fatalf("precondition: no battlefield static carries a priced raise over the hand Alien while neither applying nor granting; Festival of Embers is not set up: battlefield=%+v hand=%+v", e.G.Zone(state.ZBattlefield, 0), e.G.Zone(state.ZHand, 0))
	}

	// The raise must stay the granting statics' property only.
	_, hasRaise, ok := e.mayPlayRaiseCost(0, alienID)
	if hasRaise {
		t.Fatalf("mayPlayRaiseCost reports hasRaise for the hand Alien: Festival of Embers' PayLife<1> leaked onto a card its static neither applies to nor grants")
	}
	if !ok {
		t.Fatalf("mayPlayRaiseCost reports ok=false with no raise taken; the read failed closed without a raise")
	}

	addMana(t, e, 0, "BB")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("priority = %+v, want a priority decision for seat 0", d)
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == alienID && o.Mode == "mayplay" {
			t.Fatalf("the hand Alien Symbiosis is offered in mayplay mode off the leaked raise: %+v", o)
		}
	}
	for _, a := range e.EnsurePaymentActions() {
		if a.Cast.Object == alienID && a.Cast.CastMode() == "mayplay" {
			t.Fatalf("a payment action plans a mayplay cast of the hand Alien Symbiosis: %+v", a)
		}
	}
	if o := e.G.Obj(alienID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition broke mid-test: Alien Symbiosis = %+v, want still in hand", o)
	}
}
