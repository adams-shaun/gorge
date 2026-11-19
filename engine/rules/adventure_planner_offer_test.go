package rules

// adventure_planner_offer_test.go pins the payment planner's hand
// alternate-spell-face casts (adventure_alt, omen_alt): the offer builder
// (paymentActionsForPriority) plans them exactly as it plans the plain
// casts -- the Adventure/Omen spell face is priced with the object flipped
// to it (faceprobe.go), in the scope beginCast charges (spellScope of the
// cast mode) -- so a seat's priority extension offers the face at the same
// steps the manual walk offers it. Before this the builder skipped every
// cast option with a Mode outside {mayplay, flashback}, so the face got a
// payment action at NO step while the plain front did.

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// plannerFaceCostSwamp is an authored untapped Swamp source for the face
// planner tests below.
const plannerFaceCostSwamp = "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"

// plannerPaymentAction finds the payment action whose cast is exactly
// object/face/mode, or fails naming every offered cast.
func plannerPaymentAction(t *testing.T, actions []decision.PaymentAction, object state.ObjID, face int, mode string) decision.PaymentAction {
	t.Helper()
	var seen []string
	for _, a := range actions {
		seen = append(seen, fmt.Sprintf("{obj: %d face: %d origin: %s mode: %s}", a.Cast.Object, a.Cast.Face, a.Cast.Origin, a.Cast.CastMode()))
		if a.Cast.Object == object && a.Cast.Face == face && a.Cast.CastMode() == mode {
			return a
		}
	}
	t.Fatalf("no payment action for obj %d face %d mode %q; offered: %v", object, face, mode, seen)
	return decision.PaymentAction{}
}

// TestAdventureFacePlannerActionOfferedExecutes pins the taxed-Adventure
// planner action end to end (the TestAdventureFaceUnderTaxOfferedFromUntappedLands
// shape, on the priority-extension surface instead of the potential one):
// (a) EnsurePaymentActions offers an action for the Adventure face
// (Cast.Face 1, CastMode "adventure_alt") beside the Creature front's plain
// action; (b) the face plan prices the face-scoped tax ({2}{B}: three
// Swamps) while the front's plan prices {1}{B} (two Swamps); (c) submitting
// the face witness casts the Adventure face for real, no cast aborted.
func TestAdventureFacePlannerActionOfferedExecutes(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 7414, taxedAdventureSrc, taxWardenSrc)
	putCreature(t, e, 0, taxWardenSrc)
	var swamps []state.ObjID
	for range 5 {
		swamps = append(swamps, onBoard(t, e, 0, plannerFaceCostSwamp))
	}
	toMain1(t, e)
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand || o.Owner != 0 || int(o.FaceIdx) != 0 || adventureSpellFace(o) == nil {
		t.Fatalf("precondition: adventure card not in hand at face 0: %+v", o)
	}
	ppPriority(t, e)
	actions := e.EnsurePaymentActions()
	if len(actions) == 0 {
		t.Fatal("no payment actions offered at all")
	}
	alt := plannerPaymentAction(t, actions, id, 1, "adventure_alt")
	if alt.Label != "Cast Grim Errand" {
		t.Fatalf("face action label %q, want \"Cast Grim Errand\"", alt.Label)
	}
	if len(alt.Plans) == 0 {
		t.Fatalf("face action has no plan: %+v", alt)
	}
	front := plannerPaymentAction(t, actions, id, 0, "")
	if len(front.Plans) == 0 {
		t.Fatalf("front action has no plan: %+v", front)
	}
	faceCost, frontCost := alt.Plans[0].Cost, front.Plans[0].Cost
	if faceCost.Generic != 2 || faceCost.Mana[state.ManaIndex('B')] != 1 {
		t.Fatalf("face plan cost %+v, want the face-scoped tax {2}{B}", faceCost)
	}
	if frontCost.Generic != 1 || frontCost.Mana[state.ManaIndex('B')] != 1 {
		t.Fatalf("front plan cost %+v, want the printed {1}{B}", frontCost)
	}
	if got := len(alt.Plans[0].Activations); got != 3 {
		t.Fatalf("face witness %s (%d steps), want three Swamps", lrPlanString(alt.Plans[0].Activations), got)
	}
	if got := len(front.Plans[0].Activations); got != 2 {
		t.Fatalf("front witness %s (%d steps), want two Swamps", lrPlanString(front.Plans[0].Activations), got)
	}
	for _, plan := range []decision.PaymentPlan{alt.Plans[0], front.Plans[0]} {
		for _, a := range plan.Activations {
			found := false
			for _, src := range swamps {
				found = found || a.Source == src
			}
			if !found {
				t.Fatalf("witness tapped %d, want only the untapped Swamps", a.Source)
			}
		}
	}
	// (c) the witness executes: the Adventure face is cast for real.
	if err := e.ValidateCastPayment(0, alt.Cast, alt.Plans[0]); err != nil {
		t.Fatalf("ValidateCastPayment rejected the face witness: %v", err)
	}
	submitPaymentPlan(t, e, e.Pending(), alt)
	if hasNote(e, "cast aborted") {
		t.Fatal("the planned Adventure cast aborted")
	}
	so := e.G.Obj(id)
	if so.Zone != state.ZStack || int(so.FaceIdx) != 1 {
		t.Fatalf("cast object after submit: zone %s face %d, want stack at the Adventure face", so.Zone, so.FaceIdx)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending after face cast = %+v, want Grim Errand's target ask", d)
	}
}

// TestAdventureFacePlannerCombatTiming pins the begin-combat step: with a
// plain instant and an Adventure card (instant Adventure face) in hand and
// no sorcery timing left, BOTH the plain instant and the Adventure face get
// payment actions -- the planner follows the manual walk's timing, not the
// plain cast's.
func TestAdventureFacePlannerCombatTiming(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := miscHandsEngine(t, reg, []string{"Picklock Prankster", "Opt"}, nil, []string{"Island", "Island", "Island", "Island"}, nil)
	picklockID := state.ObjID(0)
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Picklock Prankster" {
			picklockID = id
			if adventureSpellFace(o) == nil {
				t.Fatalf("precondition: %s has no Adventure face", o.Face().Name)
			}
			if int(o.FaceIdx) != 0 || o.Owner != 0 {
				t.Fatalf("precondition: Picklock Prankster not at face 0 in hand: %+v", o)
			}
		}
	}
	if picklockID == 0 {
		t.Fatal("precondition: Picklock Prankster not in seat 0's hand")
	}
	optID := state.ObjID(0)
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Opt" {
			optID = id
		}
	}
	if optID == 0 {
		t.Fatal("precondition: Opt not in seat 0's hand")
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	driveToStep(t, e, 1, 0, state.StepBeginCombat)
	if e.G.Step != state.StepBeginCombat {
		t.Fatalf("precondition: step %s, want begin combat", e.G.Step)
	}
	ppPriority(t, e)
	actions := e.EnsurePaymentActions()
	if len(actions) == 0 {
		t.Fatal("no payment actions offered at begin combat")
	}
	plannerPaymentAction(t, actions, optID, 0, "")
	alt := plannerPaymentAction(t, actions, picklockID, 1, "adventure_alt")
	if alt.Label != "Cast Free the Fae" {
		t.Fatalf("face action label %q, want \"Cast Free the Fae\"", alt.Label)
	}
}

// TestOmenFacePlannerActionOffered pins the Omen face (the same
// appendAlternateSpellFace home as the Adventure face): the priority
// extension offers an omen_alt action priced at the Omen face's printed
// {1}{W} (two Plains), while the front's plain action prices {2}{W} (three
// Plains).
func TestOmenFacePlannerActionOffered(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 97202, faceCastOmenSrc)
	first := onBoard(t, e, 0, lrPlains)
	second := onBoard(t, e, 0, lrPlains)
	third := onBoard(t, e, 0, lrPlains)
	toMain1(t, e)
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand || o.Owner != 0 || int(o.FaceIdx) != 0 || omenSpellFace(o) == nil {
		t.Fatalf("precondition: omen card not in hand at face 0: %+v", o)
	}
	ppPriority(t, e)
	actions := e.EnsurePaymentActions()
	if len(actions) == 0 {
		t.Fatal("no payment actions offered at all")
	}
	alt := plannerPaymentAction(t, actions, id, 1, "omen_alt")
	if len(alt.Plans) == 0 {
		t.Fatalf("omen action has no plan: %+v", alt)
	}
	front := plannerPaymentAction(t, actions, id, 0, "")
	if len(front.Plans) == 0 {
		t.Fatalf("front action has no plan: %+v", front)
	}
	if c := alt.Plans[0].Cost; c.Generic != 1 || c.Mana[state.ManaIndex('W')] != 1 {
		t.Fatalf("omen plan cost %+v, want the printed {1}{W}", c)
	}
	if c := front.Plans[0].Cost; c.Generic != 2 || c.Mana[state.ManaIndex('W')] != 1 {
		t.Fatalf("front plan cost %+v, want the printed {2}{W}", c)
	}
	if got := len(alt.Plans[0].Activations); got != 2 {
		t.Fatalf("omen witness %s (%d steps), want two Plains ({1}{W} is two mana)", lrPlanString(alt.Plans[0].Activations), got)
	}
	if got := len(front.Plans[0].Activations); got != 3 {
		t.Fatalf("front witness %s (%d steps), want three Plains", lrPlanString(front.Plans[0].Activations), got)
	}
	for _, a := range alt.Plans[0].Activations {
		if a.Source != first && a.Source != second && a.Source != third {
			t.Fatalf("witness tapped %d, want one of the three Plains", a.Source)
		}
	}
}
