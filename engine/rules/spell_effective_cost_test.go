package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

const aetherVialProjectionSrc = "Name:Aether Vial\nManaCost:1\nTypes:Artifact\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

// reorderedCostProjectionSrc is a card whose printed ManaCost string is not in
// formatCost's normalized form: "2 G B" normalizes to "2 B G". No static
// touches it, so its effective cost must stay unset.
const reorderedCostProjectionSrc = "Name:Reordered Relic\nManaCost:2 G B\nTypes:Artifact\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

func TestSpellEffectiveCostThalia(t *testing.T) {
	e, _, id := newFixtureDeck(t, 9601, aetherVialProjectionSrc, taxWardenSrc)
	putCreature(t, e, 0, taxWardenSrc)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand {
		t.Fatalf("Aether Vial fixture must be in hand, got object=%+v", o)
	}
	if got := o.Face().ManaCost; got != "1" {
		t.Fatalf("printed precondition ManaCost = %q, want 1", got)
	}
	got := e.SpellEffectiveCost(0, id)
	if got != "2" || got == o.Face().ManaCost {
		t.Fatalf("SpellEffectiveCost = %q, printed = %q; want a changed effective cost 2", got, o.Face().ManaCost)
	}
	v := view.Project(e.G, e, 0, nil)
	for _, card := range v.Players[0].Hand {
		if card.ID == id {
			if card.EffectiveManaCost != "2" {
				t.Fatalf("view effective cost = %q, want 2", card.EffectiveManaCost)
			}
			return
		}
	}
	t.Fatalf("Aether Vial %d was not projected in its owner's hand", id)
}

func TestSpellEffectiveCostUntaxedOmitted(t *testing.T) {
	e, _, id := newFixtureDeck(t, 9602, aetherVialProjectionSrc)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand {
		t.Fatalf("Aether Vial fixture must be in hand, got object=%+v", o)
	}
	if got := e.SpellEffectiveCost(0, id); got != "" {
		t.Fatalf("untaxed effective cost = %q, want empty", got)
	}
	v := view.Project(e.G, e, 0, nil)
	for _, card := range v.Players[0].Hand {
		if card.ID == id {
			if card.EffectiveManaCost != "" {
				t.Fatalf("untaxed view effective cost = %q, want omitted", card.EffectiveManaCost)
			}
			return
		}
	}
	t.Fatalf("Aether Vial %d was not projected in its owner's hand", id)
}

// A printed cost whose colour order formatCost re-renders (raw "2 G B" ->
// normalized "2 B G") must NOT be reported as an effective cost when no
// RaiseCost/ReduceCost applies. The guard must compare normalized costs on both
// sides, not a normalized render against the raw printed string.
func TestSpellEffectiveCostUntaxedRearrangedPrintedOmitted(t *testing.T) {
	e, _, id := newFixtureDeck(t, 9603, reorderedCostProjectionSrc)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand {
		t.Fatalf("fixture must be in hand, got object=%+v", o)
	}
	printed := o.Face().ManaCost
	// Precondition: the printed string genuinely differs from its normalized
	// render, or this test could not exercise the raw-vs-normalized defect.
	if got := formatCost(e.parseCost(printed)); got == printed {
		t.Fatalf("precondition: printed %q already normalizes to itself; pick a reordered cost", printed)
	}
	if got := e.SpellEffectiveCost(0, id); got != "" {
		t.Fatalf("untaxed reordered effective cost = %q, want empty (printed %q)", got, printed)
	}
	v := view.Project(e.G, e, 0, nil)
	for _, card := range v.Players[0].Hand {
		if card.ID == id {
			if card.EffectiveManaCost != "" {
				t.Fatalf("untaxed reordered view effective cost = %q, want omitted", card.EffectiveManaCost)
			}
			return
		}
	}
	t.Fatalf("card %d was not projected in its owner's hand", id)
}

// TestSpellEffectiveCostVanillaGrant pins the native view against the planner
// for the defect record: a vanilla creature has no SP$ line, so the old
// "has a spell ability" guard declined it even though Absorb Energy's
// perpetual AnimateAll ReduceCost grant applies and the payment planner
// charges the reduced cost.
func TestSpellEffectiveCostVanillaGrant(t *testing.T) {
	benthic := corpusAlternativeCard(t, "Benthic Giant")
	absorb := corpusAlternativeCard(t, "Absorb Energy")
	e := handEngine(t, benthic)
	benthicID := e.G.Zone(state.ZHand, 0)[0]

	// Precondition: Benthic Giant is in the zone the rule reads, vanilla (no
	// SP$ line), and has no effective cost before the grant.
	if o := e.G.Obj(benthicID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("Benthic Giant must be in hand, got object=%+v", o)
	}
	if sa := benthic.Faces[0].SpellAbility(); sa != nil {
		t.Fatalf("precondition: Benthic Giant has SP$ %q; the vanilla shape is required", sa.API)
	}
	if got := e.SpellEffectiveCost(0, benthicID); got != "" {
		t.Fatalf("effective cost before the grant = %q, want empty", got)
	}

	absorbID := cgPlace(t, e, 0, absorb)
	countered := cgPlace(t, e, 1, card(t, "Name:Test Spell Creature\nManaCost:2\nTypes:Creature Golem\nPT:2/2\nOracle:x\n"))
	cgResolveSVar(t, e, absorbID, "DBAnimate", countered)

	printed := e.G.Obj(benthicID).Face().ManaCost
	if printed != "5 U" {
		t.Fatalf("printed ManaCost = %q, want 5 U", printed)
	}
	got := e.SpellEffectiveCost(0, benthicID)
	if got == printed {
		t.Fatalf("effective cost = printed %q; the granted reduction did not apply", printed)
	}
	if got != "4 U" {
		t.Fatalf("SpellEffectiveCost = %q, want 4 U (printed %q)", got, printed)
	}

	v := view.Project(e.G, e, 0, nil)
	projected := false
	for i := range v.Players[0].Hand {
		card := &v.Players[0].Hand[i]
		if card.ID != benthicID {
			continue
		}
		projected = true
		if card.ManaCost != "5 U" {
			t.Fatalf("view mana_cost = %q, want 5 U", card.ManaCost)
		}
		if card.EffectiveManaCost != "4 U" {
			t.Fatalf("view effective_mana_cost = %q, want 4 U", card.EffectiveManaCost)
		}
	}
	if !projected {
		t.Fatalf("Benthic Giant %d was not projected in its owner's hand", benthicID)
	}

	// The planner already charges the same 4U; the view must agree with it.
	e.G.Players[0].Pool = state.Mana{}
	e.G.Players[0].Pool[state.MU] = 5
	e.pending = nil
	e.askPriority(0)
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("priority decision not posed: %+v", d)
	}
	planned := false
	for _, a := range e.EnsurePaymentActions() {
		if a.Cast.Object != benthicID || len(a.Plans) == 0 {
			continue
		}
		planned = true
		if a.Plans[0].Cost.Generic != 4 {
			t.Fatalf("plan generic = %d, want 4", a.Plans[0].Cost.Generic)
		}
		if want := (decision.ManaAmount{0, 1, 0, 0, 0, 0}); a.Plans[0].Cost.Mana != want {
			t.Fatalf("plan mana = %v, want %v", a.Plans[0].Cost.Mana, want)
		}
	}
	if !planned {
		t.Fatalf("Benthic Giant %d was not offered a payment plan with 5 blue", benthicID)
	}
}

// TestSpellEffectiveCostNoManaCostStaysOmitted is the negative control for the
// vanilla-creature fix: a face with no printed mana cost has no plain cast to
// price, even when a live RaiseCost static matches it. A bare deletion of the
// old guard would make this land report "1".
func TestSpellEffectiveCostNoManaCostStaysOmitted(t *testing.T) {
	land := corpusAlternativeCard(t, "Karplusan Forest")
	e := handEngine(t, land, card(t, aetherVialProjectionSrc))
	landID := e.G.Zone(state.ZHand, 0)[0]

	if o := e.G.Obj(landID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("Karplusan Forest must be in hand, got object=%+v", o)
	}
	if !isNoManaCost(land.Faces[0].ManaCost) {
		t.Fatalf("precondition: Karplusan Forest ManaCost = %q, want no cost", land.Faces[0].ManaCost)
	}
	if sa := land.Faces[0].SpellAbility(); sa != nil {
		t.Fatalf("precondition: Karplusan Forest has SP$ %q; the no-SP$ shape is required", sa.API)
	}
	cgPlace(t, e, 0, card(t, taxWardenSrc))
	// Precondition: the tax is live -- it prices another noncreature card in
	// hand, so the land assertion below cannot pass on an inert static.
	if got := e.SpellEffectiveCost(0, e.G.Zone(state.ZHand, 0)[1]); got != "2" {
		t.Fatalf("tax precondition: Aether Vial effective cost = %q, want 2", got)
	}

	if got := e.SpellEffectiveCost(0, landID); got != "" {
		t.Fatalf("no-mana-cost effective cost = %q, want empty", got)
	}
	v := view.Project(e.G, e, 0, nil)
	projected := false
	for i := range v.Players[0].Hand {
		card := &v.Players[0].Hand[i]
		if card.ID != landID {
			continue
		}
		projected = true
		if card.EffectiveManaCost != "" {
			t.Fatalf("no-mana-cost view effective_mana_cost = %q, want omitted", card.EffectiveManaCost)
		}
	}
	if !projected {
		t.Fatalf("Karplusan Forest %d was not projected in its owner's hand", landID)
	}
}
