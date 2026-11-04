package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The three fixtures pin the Tifa's Limit Break shape this test's cause was
// found on: a modal (Tiered) spell whose printed cost is one coloured mana,
// a granted any-colour mana ability that is the only source of that colour,
// and a permanent that prints no mana ability of its own.
const (
	mcPrefixRealm = "Name:Prefix Realm\nManaCost:4 G G\nTypes:Enchantment\n" +
		"S:Mode$ Continuous | Affected$ Permanent.Other+YouCtrl | AddAbility$ AnyMana | Description$ Fixture grant.\n" +
		"SVar:AnyMana:AB$ Mana | Cost$ T | Produced$ Any | SpellDescription$ Fixture tap for any colour.\nOracle:x\n"
	mcPrefixRelic = "Name:Prefix Relic\nTypes:Artifact\nOracle:x\n"
	mcPrefixSpell = "Name:Prefix Tiered\nManaCost:G\nTypes:Instant\nK:Tiered\n" +
		"A:SP$ Charm | Choices$ DBFree,DBCostly | MinCharmNum$ 1\n" +
		"SVar:DBFree:DB$ Draw | ModeCost$ 0 | Defined$ You | NumCards$ 1 | SpellDescription$ Free mode.\n" +
		"SVar:DBCostly:DB$ Draw | ModeCost$ 2 | Defined$ You | NumCards$ 1 | SpellDescription$ Costly mode.\nOracle:x\n"
	// No mode is free: the cheapest total is the printed {R} plus one {2}.
	mcPrefixPaidSpell = "Name:Prefix Paid Tiered\nManaCost:R\nTypes:Instant\nK:Tiered\n" +
		"A:SP$ Charm | Choices$ DBPaidOne,DBPaidTwo | MinCharmNum$ 1\n" +
		"SVar:DBPaidOne:DB$ Draw | ModeCost$ 2 | Defined$ You | NumCards$ 1 | SpellDescription$ Paid one.\n" +
		"SVar:DBPaidTwo:DB$ Draw | ModeCost$ 2 | Defined$ You | NumCards$ 1 | SpellDescription$ Paid two.\nOracle:x\n"
)

// TestModalCostCastIsPricedThroughAGrantedManaPrefix is the paid half of the
// Tifa's Limit Break diagnosis: a modal cast's printed cost is payable only
// through a granted any-colour ability (the planner's census gap), so the
// cast planner declines the shape (shape:modal_cost) and the composed plan
// must still be tried, found insufficient with a gap, and scripted by
// tapping the granted source first. Before the fix the cast stayed
// "unsupported shape:modal_cost" and the play vanished from the autopay
// vocabulary.
func TestModalCostCastIsPricedThroughAGrantedManaPrefix(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9600, mcPrefixSpell)
	onBoard(t, e, 0, mcPrefixRealm)
	relic := onBoard(t, e, 0, mcPrefixRelic)

	// Precondition: the granted any-colour ability is really activatable from
	// the relic, and the empty pool offers neither the cast nor its mana.
	if !hasModalPrefixActivate(e.legalActions(0), relic) {
		t.Fatalf("the granted mana ability is not offered: %+v", e.legalActions(0))
	}
	d := ppPriority(t, e)
	if hasCastOption(d.Options, spell) {
		t.Fatalf("the tiered cast should not be in the empty-pool options: %+v", d.Options)
	}
	// Precondition: this is the modal-cost decline, not some other shape.
	if detail := e.paymentPlanCastShapeDetail(0, spell); detail != "shape:modal_cost" {
		t.Fatalf("shape detail = %q, want shape:modal_cost", detail)
	}
	// Precondition: the relic is the planner census's uncovered source, so a
	// plan at the printed cost cannot pay without the prefix.
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || !slices.Contains(census.gaps, relic) {
		t.Fatalf("census complete=%v gaps=%v, want the relic %d as a gap", census.complete, census.gaps, relic)
	}
	// Precondition: the free mode is what makes the printed-cost plan an
	// exact total-cost witness.
	if !e.potentialModalCastFreeMode(0, spell) {
		t.Fatal("the tiered fixture's free mode should be legal")
	}

	pp := ppFind(t, e.PotentialPaymentPlans(0), "cast", spell)
	if pp.Reason != "" {
		t.Fatalf("tiered cast verdict = %q %q, want a priced play", pp.Reason, pp.Detail)
	}
	if pp.Plan == nil && len(pp.Script) == 0 {
		t.Fatalf("tiered cast priced with neither a plan nor a script: %+v", pp)
	}
}

func hasModalPrefixActivate(opts []decision.Option, obj state.ObjID) bool {
	for _, o := range opts {
		if o.Kind == "activate" && o.Obj == obj {
			return true
		}
	}
	return false
}

// TestModalCostCastWithoutAFreeModeStaysUnpriced is the over-offer half of the
// same diagnosis: when every legal mode charges extra, a printed-cost plan is
// not a witness -- the cast's minimum total is the printed cost plus the
// cheapest ModeCost$, so a plan or script that covers only the printed cost
// must not price it. The granted any-colour source makes the printed pip
// payable (and is the census gap a script would tap) but cannot fund a {2}
// mode, which is exactly the OTJ Explosive Derailment divergence.
func TestModalCostCastWithoutAFreeModeStaysUnpriced(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9601, mcPrefixPaidSpell)
	onBoard(t, e, 0, mcPrefixRealm)
	onBoard(t, e, 0, mcPrefixRelic)
	ppPriority(t, e)
	if detail := e.paymentPlanCastShapeDetail(0, spell); detail != "shape:modal_cost" {
		t.Fatalf("shape detail = %q, want shape:modal_cost", detail)
	}
	if e.potentialModalCastFreeMode(0, spell) {
		t.Fatal("no mode of the paid fixture is free")
	}
	pp := ppFind(t, e.PotentialPaymentPlans(0), "cast", spell)
	if pp.Reason == "" || pp.Plan != nil || len(pp.Script) != 0 {
		t.Fatalf("cast with no free mode was priced: %+v", pp)
	}
}
