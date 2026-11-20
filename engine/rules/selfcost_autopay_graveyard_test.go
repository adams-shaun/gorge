package rules

// selfcost_autopay_graveyard_test.go pins the autopay builder's pricing of a
// card's OWN printed-face ReduceCost while the same build also prices the
// card's Adventure face. The builder's alternate-face arm runs inside the face
// probe (faceprobe.go), and its holds-on-stack check opens a nested Derived
// scope inside the spell probe (castprobe.go); a nested walk must not refill
// the shared board-statics cache with the probed board, or the plain cast the
// builder prices next loses the self-cost static and is dropped as unpayable
// (Hearth Elemental, std-woe pair 173, seed 434573, decision 324).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// redDorkSrc is a non-intrinsic red mana source: the payment plan for the
// Adventure face names it, so paymentPlanHoldsOnStack runs its census inside
// the spell probe instead of skipping every intrinsic basic-land ability.
const redDorkSrc = "Name:Test Red Dork\nManaCost:1\nTypes:Creature Elf\nPT:1/1\n" +
	"A:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Add one red test mana.\nOracle:x\n"

// TestSelfCostAutopayPlannerPricesGraveyardAdventureCount drives the autopay
// offer builder (paymentActionsForPriority) rather than the offer gate or
// PlanCastPayment: the hand's plain casts and Adventure faces are priced in
// ONE build. A plain cast ahead of the Adventure card pre-warms the build's
// one captured cost-static source, and the Adventure face is listed above its
// own printed cast, so the probe runs between the two. The printed-cast
// action must survive with the two-card-reduced {3}{R}: four red dorks pay it,
// while the raw {5}{R} would need six.
func TestSelfCostAutopayPlannerPricesGraveyardAdventureCount(t *testing.T) {
	e := mayPlayBase(t)
	filler := handCard(e, card(t, advFiller), 0)
	id := handCard(e, card(t, hearthLike), 0)
	graveCard(e, card(t, advFiller), 0, 0)
	graveCard(e, card(t, advFiller), 0, 0)
	for i := 0; i < 4; i++ {
		onBoardReady(t, e, 0, redDorkSrc)
	}
	addMana(t, e, 0, "")
	// Preconditions the plan below rests on: the printed face is shown, the
	// two count terms are in the graveyard, and the only mana is four
	// non-intrinsic red sources (so the Adventure plan's holds-on-stack census
	// runs inside the probe).
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand || int(o.FaceIdx) != 0 || len(e.G.Zone(state.ZGraveyard, 0)) != 2 || len(e.G.Zone(state.ZHand, 0)) != 2 {
		t.Fatalf("precondition: card not in hand at the printed face with two graveyard cards: %+v", o)
	}
	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != 4 || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: battlefield=%d pool=%v, want four sources and an empty pool", got, e.G.Players[0].Pool)
	}
	actions := e.EnsurePaymentActions()
	// The plain cast ahead of the Adventure card pre-warms the build's one
	// captured cost-static source; the Adventure probe below refills that
	// source's backing unless the walk caches stand down inside the probe.
	plannerPaymentAction(t, actions, filler, 0, "")
	// The Adventure face is planned too: its holds-on-stack probe is the one
	// that opens the nested scope this test guards.
	alt := plannerPaymentAction(t, actions, id, 1, "adventure_alt")
	if len(alt.Plans[0].Activations) == 0 {
		t.Fatalf("precondition: Adventure plan has no activations: %+v", alt.Plans[0])
	}
	for _, pa := range alt.Plans[0].Activations {
		if pa.Ability.Kind == decision.PaymentAbilityIntrinsic {
			t.Fatalf("precondition: Adventure plan activation is intrinsic: %+v", pa)
		}
	}
	front := plannerPaymentAction(t, actions, id, 0, "")
	if front.Plans[0].Cost.Generic != 3 || front.Plans[0].Cost.Mana[state.ManaIndex('R')] != 1 {
		t.Fatalf("printed plan cost %+v, want the two-card-reduced {3}{R}", front.Plans[0].Cost)
	}
	// Precondition for the reduction: with an empty graveyard the raw {5}{R}
	// stays beyond four red sources, so the builder must not offer a printed
	// plan.
	e2 := mayPlayBase(t)
	id2 := handCard(e2, card(t, hearthLike), 0)
	for i := 0; i < 4; i++ {
		onBoardReady(t, e2, 0, redDorkSrc)
	}
	addMana(t, e2, 0, "")
	for _, a := range e2.EnsurePaymentActions() {
		if a.Cast.Object == id2 && a.Cast.Face == 0 && a.Cast.CastMode() == "" {
			t.Fatalf("printed cast planned with an empty graveyard: %+v", a)
		}
	}
}

// manaReachProbeSrc carries a mana-reaching cost static on its front face and
// none on its Adventure face, so a probe of the back face must answer
// "no mana-reaching static" without serving the front-face memo.
const manaReachProbeSrc = "Name:Test Reach\nManaCost:2\nTypes:Creature Elemental\nPT:2/2\nAlternateMode:Adventure\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Activated | Amount$ 1 | EffectZone$ All | Description$ x\nOracle:x\n" +
	"\nALTERNATE\n\n" +
	"Name:Test Reach Back\nManaCost:1 U\nTypes:Sorcery Adventure\nA:SP$ Draw | NumCards$ 1\nOracle:y\n"

// TestSelfCostManaReachingMemoStandsDownInProbe pins the per-state
// mana-reaching memo's probe edge (mana_cost_adjust.go): the memo is not keyed
// on the probed face, so a probe must scan instead of serving the live-face
// answer it stored, and must not overwrite the live-face answer with the
// probe's. The front face's Type$ Activated reduction is mana-reaching; the
// Adventure face has no static.
func TestSelfCostManaReachingMemoStandsDownInProbe(t *testing.T) {
	e := mayPlayBase(t)
	id := handCard(e, card(t, manaReachProbeSrc), 0)
	// Pre-warm the per-state memo at the live face.
	if !e.anyManaReachingCostStatic() {
		t.Fatal("precondition: the front face's mana-reaching static was not collected")
	}
	o := e.G.Obj(id)
	alt := o.Card.Faces[1]
	if alt == nil || alt == o.Card.Faces[0] {
		t.Fatal("precondition: fixture has no Adventure face")
	}
	got := true
	if !e.offerAsFace(id, alt, func() bool {
		got = e.anyManaReachingCostStatic()
		return true
	}) {
		t.Fatal("offerAsFace refused the fixture's own back face")
	}
	if got {
		t.Fatal("probe served the live-face mana-reaching answer")
	}
	if !e.anyManaReachingCostStatic() {
		t.Fatal("live-face mana-reaching answer was not restored after the probe")
	}
}
