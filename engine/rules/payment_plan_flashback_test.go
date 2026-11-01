package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Payment plans for the two shapes the auto-pay bot used to tap for by hand
// (botmanual, 2026-10-14): a graveyard card's flashback cast
// (decision.OriginFlashback) and a fixed-count discard additional cost.
// Measured before the fix, flashback casts were the largest planner gap in
// both a network bot vs sb-tactical sample (Think Twice) and the constructed
// repo-deck auto-pay mirror (Pursue the Past), and Thrill of Possibility's
// discard the next.

const (
	paymentPlanIsland  = "Name:Island\nTypes:Basic Land Island\nOracle:x\n"
	flashbackPlanShape = "Name:Planned Flashback Shape\nManaCost:1 U\nTypes:Instant\nK:Flashback:2 U\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
)

// toGraveyard moves the fixture spell from its owner's hand to the graveyard.
func toGraveyard(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: spell zone = %s, want graveyard", z)
	}
}

// TestPaymentPlanOffersFlashbackCast: a graveyard card whose flashback cost
// the seat's sources pay gets a one-click action with OriginFlashback whose
// witness pays the FLASHBACK cost ({2}{U}), not the printed one ({1}{U});
// submitting it casts the card in Mode "flashback" with exactly the witness's
// sources tapped and nothing left floating.
func TestPaymentPlanOffersFlashbackCast(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9701, flashbackPlanShape)
	islands := []state.ObjID{onBoard(t, e, 0, paymentPlanIsland), onBoard(t, e, 0, paymentPlanIsland), onBoard(t, e, 0, paymentPlanIsland)}
	toGraveyard(t, e, spell)

	d := paymentPlanReask(t, e)
	a := paymentPlanActionFor(t, d, spell)
	if a.Cast.Origin != decision.OriginFlashback {
		t.Fatalf("action origin = %q, want %q", a.Cast.Origin, decision.OriginFlashback)
	}
	if got := a.Plans[0].Cost; got.Generic != 2 || got.Mana[state.ManaIndex('U')] != 1 {
		t.Fatalf("plan cost = %+v, want the flashback cost {2}{U}", got)
	}
	if len(a.Plans[0].Activations) != 3 {
		t.Fatalf("plan activations = %d, want the three Islands", len(a.Plans[0].Activations))
	}
	if !d.PotentialCastsKnown || !containsObjID(d.PotentialCasts, spell) {
		t.Errorf("cast census known=%v %v, want the flashback card listed", d.PotentialCastsKnown, d.PotentialCasts)
	}

	submitPaymentPlan(t, e, d, a)
	if nd := e.Pending(); nd == nil || nd.Kind != decision.KPriority {
		t.Fatalf("pending = %s, want priority after the settled cast", paymentPlanPendingSummary(nd))
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Fatalf("spell zone = %s, want stack", z)
	}
	for _, id := range islands {
		if !e.G.Obj(id).Tapped {
			t.Errorf("Island %d untapped: the witness pays {2}{U} from all three", id)
		}
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Errorf("pool after payment = %d, want 0", got)
	}
	// Resolving a flashback spell exiles it (CR 702.34a): the cast really
	// was the flashback cast, not a plain graveyard cast.
	resolveStack(t, e)
	if z := e.G.Obj(spell).Zone; z != state.ZExile {
		t.Errorf("resolved spell zone = %s, want exile (flashback)", z)
	}
}

// TestPaymentPlanFlashbackInsufficientNoPlan: two Islands pay the printed
// {1}{U} but not the flashback {2}{U}, so no flashback action is offered --
// the witness is priced at the cost beginCast charges, never the printed one.
func TestPaymentPlanFlashbackInsufficientNoPlan(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9702, flashbackPlanShape)
	onBoard(t, e, 0, paymentPlanIsland)
	onBoard(t, e, 0, paymentPlanIsland)
	toGraveyard(t, e, spell)
	d := paymentPlanReask(t, e)
	for _, a := range d.PaymentActions {
		if a.Cast.Object == spell && len(a.Plans) > 0 {
			t.Fatalf("offered %+v, want no plan for an unaffordable flashback", a)
		}
	}
	if got := e.PlanCastPayment(0, decision.PlannedCast{Object: spell, Origin: decision.OriginFlashback}); got.Plan != nil {
		t.Fatalf("planner outcome = %+v, want no witness", got)
	}
}

// TestPaymentPlanFlashbackOriginMustMatchZone: a flashback origin names a
// graveyard card; the same card in hand is not a flashback cast.
func TestPaymentPlanFlashbackOriginMustMatchZone(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9703, flashbackPlanShape)
	for i := 0; i < 3; i++ {
		onBoard(t, e, 0, paymentPlanIsland)
	}
	if got := e.PlanCastPayment(0, decision.PlannedCast{Object: spell, Origin: decision.OriginFlashback}); got.Plan != nil || got.Reason != "unsupported" {
		t.Fatalf("hand card planned as flashback: %+v, want unsupported", got)
	}
}

// TestPaymentPlanSeeksDiscardOffer: a Thrill-of-Possibility-shaped cast
// (Cost$ 1 R Discard<1/Card>) gets a mana-only witness; the discard is the
// cast's own in-flow ask after submit, exactly as the fixed sacrifice is.
func TestPaymentPlanSeeksDiscardOffer(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9704, "Name:Planned Thrill Shape\nManaCost:1 R\nTypes:Instant\nA:SP$ Draw | Cost$ 1 R Discard<1/Card> | NumCards$ 2\nOracle:x\n")
	m1, m2 := onBoard(t, e, 0, paymentPlanMountain), onBoard(t, e, 0, paymentPlanMountain)
	hand := e.G.Zone(state.ZHand, 0)
	var fodder state.ObjID
	for _, id := range hand {
		if id != spell {
			fodder = id
			break
		}
	}
	if fodder == 0 {
		t.Fatal("precondition: no other card in hand to discard")
	}
	if detail := e.paymentPlanCastShapeDetail(0, spell); detail != "" {
		t.Fatalf("shape detail = %q, want the fixed-count discard admitted", detail)
	}
	d := paymentPlanReask(t, e)
	a := paymentPlanActionFor(t, d, spell)
	submitPaymentPlan(t, e, d, a)
	paymentPlanChooseObj(t, e, fodder)
	if nd := e.Pending(); nd == nil || nd.Kind != decision.KPriority {
		t.Fatalf("pending = %s, want priority after the settled cast", paymentPlanPendingSummary(nd))
	}
	if z := e.G.Obj(fodder).Zone; z != state.ZGraveyard {
		t.Errorf("discarded card zone = %s, want graveyard", z)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Errorf("spell zone = %s, want stack", z)
	}
	if !e.G.Obj(m1).Tapped || !e.G.Obj(m2).Tapped {
		t.Error("the witness's Mountains were not both tapped")
	}
}

// TestPaymentPlanDiscardShapeBoundary: a variable discard and a discard beside
// another non-mana part stay withheld.
func TestPaymentPlanDiscardShapeBoundary(t *testing.T) {
	t.Parallel()
	for i, face := range []string{
		"A:SP$ Draw | Cost$ R Discard<X/Card> | NumCards$ 1",
		"A:SP$ Draw | Cost$ R Discard<1/Card> PayLife<1> | NumCards$ 1",
	} {
		e, _, spell := newFixtureDeck(t, uint64(9710+i), "Name:Discard Boundary\nManaCost:R\nTypes:Instant\n"+face+"\nOracle:x\n")
		onBoard(t, e, 0, paymentPlanMountain)
		if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil {
			t.Errorf("%s: planned %+v, want withheld", face, got)
		}
	}
}
