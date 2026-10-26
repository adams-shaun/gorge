package rules

// CR 106.6: CantCastNonArtifactSpells and CantCastSpellFromHand restrict only
// the spending they name — a spell cast. Both terms must leave an activated
// ability's mana payment free. These pins drive the real corpus producers'
// restricted batches into a real {1} activated ability (the dotlessClerk
// fixture), asserting the pool view and the priority decision, then that the
// batch is consumed.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestKr8RestrictValidDotlessPaymentTermsAbilityPayments pins the non-spell
// side of the two "can't cast" dotless terms on their real corpus producers:
// Hydraulic Helper's CantCastNonArtifactSpells {U} and Heartwood Crafter's
// CantCastSpellFromHand {C} both pay a nonartifact permanent's {1} activated
// ability, and the restricted batch is consumed by that payment.
func TestKr8RestrictValidDotlessPaymentTermsAbilityPayments(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)

	t.Run("CantCastNonArtifactSpells_PaysActivatedAbility", func(t *testing.T) {
		t.Parallel()
		hh := searchCorpusCard(t, reg, "Hydraulic Helper")
		e, cfg := dotlessEngine(t, reg, hh, card(t, dotlessClerk))
		src := searchMoveByName(t, e, "Hydraulic Helper", state.ZBattlefield)
		// A real logged TurnChange (not a raw SummonSick write) clears CR
		// 302.6 sickness on the log the replayCheck below reads from.
		e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
		clerk := searchMoveByName(t, e, "Sunrise Clerk", state.ZBattlefield)
		if o := e.G.Obj(clerk); o == nil || o.Face() == nil || o.Face().IsArtifact() {
			t.Fatalf("test precondition: the payer must be a nonartifact permanent: %+v", o)
		}

		tapForRestrictedBatch(t, e, src, "CantCastNonArtifactSpells")
		batches := restrictedBatchesOf(e, 0, "CantCastNonArtifactSpells")
		if len(batches) != 1 || batches[0].Amount != 1 {
			t.Fatalf("test precondition: Hydraulic Helper's batch(es) = %+v, want one 1-unit batch", batches)
		}

		// The {1} activated ability is a non-spell payment, so the restricted
		// {U} must be visible to it.
		if got := pay.AvailableFor(asPayer(e), 0, paymentFor(clerk, true, e.parseCost("1"))).Pool.Total(); got != 1 {
			t.Fatalf("manaAvailableFor({1} activation) = %d, want 1 (CantCastNonArtifactSpells leaves non-spell payments free)", got)
		}
		e.pending = nil
		e.priorityRound()
		abIdx := findOption(e.Pending(), "ability", clerk)
		if abIdx < 0 {
			t.Fatalf("the nonartifact permanent's {1} activation was not offered on the restricted {U}: %+v", e.Pending())
		}
		submitChoices(t, e, abIdx)
		passUntilStackEmpty(t, e, 30)
		if got := restrictedBatchesOf(e, 0, "CantCastNonArtifactSpells"); len(got) != 0 {
			t.Fatalf("after the {1} activation payment the batch(es) = %+v, want none (consumed from the restricted batch)", got)
		}
		if got := e.G.Players[0].Pool.Total(); got != 0 {
			t.Fatalf("after the {1} activation payment pool total = %d, want 0", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("CantCastSpellFromHand_PaysActivatedAbility", func(t *testing.T) {
		t.Parallel()
		hc := searchCorpusCard(t, reg, "Heartwood Crafter")
		e, cfg := dotlessEngine(t, reg, hc, card(t, dotlessClerk))
		src := searchMoveByName(t, e, "Heartwood Crafter", state.ZBattlefield)
		// A real logged TurnChange (not a raw SummonSick write) clears CR
		// 302.6 sickness on the log the replayCheck below reads from.
		e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
		clerk := searchMoveByName(t, e, "Sunrise Clerk", state.ZBattlefield)

		tapForRestrictedBatch(t, e, src, "CantCastSpellFromHand")
		batches := restrictedBatchesOf(e, 0, "CantCastSpellFromHand")
		if len(batches) != 1 || batches[0].Amount != 1 {
			t.Fatalf("test precondition: Heartwood Crafter's batch(es) = %+v, want one 1-unit {C} batch", batches)
		}

		// The {1} activated ability is not a hand cast, so the restricted {C}
		// must be visible to it.
		if got := pay.AvailableFor(asPayer(e), 0, paymentFor(clerk, true, e.parseCost("1"))).Pool.Total(); got != 1 {
			t.Fatalf("manaAvailableFor({1} activation) = %d, want 1 (CantCastSpellFromHand leaves non-spell payments free)", got)
		}
		e.pending = nil
		e.priorityRound()
		abIdx := findOption(e.Pending(), "ability", clerk)
		if abIdx < 0 {
			t.Fatalf("the {1} activation was not offered on the restricted {C}: %+v", e.Pending())
		}
		submitChoices(t, e, abIdx)
		passUntilStackEmpty(t, e, 30)
		if got := restrictedBatchesOf(e, 0, "CantCastSpellFromHand"); len(got) != 0 {
			t.Fatalf("after the {1} activation payment the batch(es) = %+v, want none (consumed from the restricted batch)", got)
		}
		if got := e.G.Players[0].Pool.Total(); got != 0 {
			t.Fatalf("after the {1} activation payment pool total = %d, want 0", got)
		}
		replayCheck(t, e, cfg)
	})
}
