package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// paymentPlanAltsBoard is a seat-0 priority decision over the given sources
// with a spell costing cost in hand, its ordinary extension built.
func paymentPlanAltsBoard(t *testing.T, seed uint64, cost string, srcs ...string) (*Engine, *decision.Decision, state.ObjID, []state.ObjID) {
	t.Helper()
	e, _, spell := newFixtureDeck(t, seed, srchSpell(cost))
	var ids []state.ObjID
	for _, src := range srcs {
		if src == srchDork {
			ids = append(ids, onBoardReady(t, e, 0, src))
			continue
		}
		ids = append(ids, onBoard(t, e, 0, src))
	}
	return e, paymentPlanReask(t, e), spell, ids
}

// tappedBy is the sources a plan activates.
func tappedBy(plan decision.PaymentPlan) []state.ObjID {
	var out []state.ObjID
	for _, a := range plan.Activations {
		out = append(out, a.Source)
	}
	slices.Sort(out)
	return out
}

// {B}{R} over three Mountains and two Swamps leaves one residual (a Mountain
// and a Swamp tapped whichever ones), so one plan; {1}{B} leaves two (Swamp
// and Mountain, or both Swamps), so two. Plan 0 is the ordinary extension's
// own plan, and the ordinary extension never carries the alternatives. {2}
// leaves three: two Mountains, two Swamps, or one of each.
func TestPaymentPlanAlternativesDifferByResidual(t *testing.T) {
	t.Parallel()
	board := []string{srchMountain, srchMountain, srchMountain, srchSwamp, srchSwamp}
	for _, tc := range []struct {
		cost  string
		plans int
	}{{"B R", 1}, {"1 B", 2}, {"1 R", 2}, {"2", 3}} {
		e, d, spell, _ := paymentPlanAltsBoard(t, 9301, tc.cost, board...)
		ordinary := decision.ClonePaymentPlan(paymentPlanActionFor(t, d, spell).Plans[0])
		if n := len(paymentPlanActionFor(t, d, spell).Plans); n != 1 {
			t.Fatalf("%s: ordinary extension carries %d plans, want 1", tc.cost, n)
		}
		e.EnsurePaymentPlanAlternatives()
		a := paymentPlanActionFor(t, e.Pending(), spell)
		if len(a.Plans) != tc.plans {
			t.Fatalf("%s: %d plans %+v, want %d", tc.cost, len(a.Plans), a.Plans, tc.plans)
		}
		if !a.Plans[0].Equal(&ordinary) {
			t.Fatalf("%s: plan 0 moved: %+v, ordinary %+v", tc.cost, a.Plans[0], ordinary)
		}
		for i := 1; i < len(a.Plans); i++ {
			if slices.Equal(tappedBy(a.Plans[i]), tappedBy(a.Plans[0])) {
				t.Fatalf("%s: plan %d taps plan 0's sources", tc.cost, i)
			}
		}
	}
}

// Submitting an alternative casts with it: {1}{B}'s second plan taps both
// Swamps and keeps every Mountain. The submitted engine never built the
// alternatives itself; Submit builds them for the plan it is handed.
func TestPaymentPlanAlternativeSubmits(t *testing.T) {
	t.Parallel()
	e, d, spell, ids := paymentPlanAltsBoard(t, 9302, "1 B", srchMountain, srchMountain, srchMountain, srchSwamp, srchSwamp)
	probe := e.Clone()
	probe.EnsurePaymentPlanAlternatives()
	a := paymentPlanActionFor(t, probe.Pending(), spell)
	if len(a.Plans) != 2 {
		t.Fatalf("%d plans, want 2", len(a.Plans))
	}
	swamps := ids[3:]
	if !slices.Equal(tappedBy(a.Plans[1]), swamps) {
		t.Fatalf("plan 1 taps %v, want the Swamps %v", tappedBy(a.Plans[1]), swamps)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Payment: &decision.PaymentSelection{ActionID: a.ID, Plan: a.Plans[1]}}); err != nil {
		t.Fatalf("Submit plan 1: %v", err)
	}
	if e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("spell in %s, want the stack", e.G.Obj(spell).Zone)
	}
	for i, id := range ids {
		if want := i >= 3; e.G.Obj(id).Tapped != want {
			t.Fatalf("source %d tapped=%v, want %v", i, e.G.Obj(id).Tapped, want)
		}
	}
}

// A mana creature tapped instead of a land is another residual (the
// creature can still attack or block): {G} over a Forest and a mana elf is
// two plans, the land first.
func TestPaymentPlanAlternativeTapsTheCreature(t *testing.T) {
	t.Parallel()
	e, _, spell, ids := paymentPlanAltsBoard(t, 9303, "G", srchForest, srchDork)
	e.EnsurePaymentPlanAlternatives()
	a := paymentPlanActionFor(t, e.Pending(), spell)
	if len(a.Plans) != 2 || !slices.Equal(tappedBy(a.Plans[0]), ids[:1]) || !slices.Equal(tappedBy(a.Plans[1]), ids[1:]) {
		t.Fatalf("plans %+v, want the Forest then the elf", a.Plans)
	}
}
