package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

const potentialTurnUpForest = "Name:Forest\nTypes:Basic Land Forest\nA:AB$ Mana | Cost$ T | Produced$ G\nOracle:x\n"

// potentialTurnUpLandsEngine seats the hand cards and n untapped Forests for
// seat 0 at its main phase, with an empty floating pool.
func potentialTurnUpLandsEngine(t *testing.T, n int, hand ...string) *Engine {
	t.Helper()
	var cs []*cards.Card
	for _, h := range hand {
		cs = append(cs, card(t, h))
	}
	e := handEngine(t, cs...)
	var ids []state.ObjID
	for i := 0; i < n; i++ {
		l := e.G.AddObject(card(t, potentialTurnUpForest), 0)
		l.Zone = state.ZBattlefield
		ids = append(ids, l.ID)
	}
	e.G.SetZone(state.ZBattlefield, 0, ids)
	e.G.Step, e.G.Active = state.StepMain1, 0
	return e
}

// potentialTurnUpPlans is the potential walk's turn_face_up plans in mode.
func potentialTurnUpPlans(e *Engine, mode string) []PotentialPlan {
	var out []PotentialPlan
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "turn_face_up" && pp.Action.Mode == mode {
			out = append(out, pp)
		}
	}
	return out
}

// TestTurnFaceUpIsAPotentialPlay: a face-down permanent whose turn-up cost its
// controller's untapped lands can pay is a potential play with a payment plan,
// though nothing floats, whatever procedure turned it down (CR 708.6,
// 701.40b, 701.58b). This is the witness the oracle projects so the sweep can
// turn a synthetic carrier up after the face-down cast spent the pool.
func TestTurnFaceUpIsAPotentialPlay(t *testing.T) {
	t.Parallel()
	const morph = "Name:Hider\nManaCost:5 G G\nTypes:Creature Beast\nPT:5/5\nK:Morph:2 G\nOracle:x\n"
	const plain = "Name:Plain\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	for _, tc := range []struct {
		name, script, mode string
		set                func(o *state.Object)
		sources            int
	}{
		{"morph", morph, "", func(o *state.Object) { o.CastFlags = state.FlagMorphed }, 3},
		{"megamorph", strings.Replace(morph, "K:Morph", "K:Megamorph", 1), "", func(o *state.Object) { o.CastFlags = state.FlagMegamorphed }, 3},
		{"disguise", strings.Replace(morph, "K:Morph", "K:Disguise", 1), "", func(o *state.Object) { o.CastFlags = state.FlagDisguised }, 3},
		{"manifest", plain, "manifest", func(o *state.Object) { o.Manifested = true }, 2},
		{"cloak", plain, "cloak", func(o *state.Object) { o.Cloaked = true }, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := potentialTurnUpLandsEngine(t, tc.sources, tc.script)
			id := e.G.Zone(state.ZHand, 0)[0]
			o := e.G.Obj(id)
			e.G.SetZone(state.ZHand, 0, nil)
			o.Zone, o.FaceDown, o.Controller = state.ZBattlefield, true, 0
			tc.set(o)
			e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), id))
			for _, opt := range e.legalActions(0) {
				if opt.Kind == "turn_face_up" {
					t.Fatalf("precondition: the empty pool already offers the turn-up: %+v", opt)
				}
			}
			plans := potentialTurnUpPlans(e, tc.mode)
			if len(plans) != 1 {
				t.Fatalf("the potential walk lists %d %q turn-ups, want 1", len(plans), tc.mode)
			}
			pp := plans[0]
			if pp.Reason != "" || pp.Plan == nil || len(pp.Plan.Activations) != tc.sources {
				t.Fatalf("no %d-source witness: reason %q detail %q plan %+v", tc.sources, pp.Reason, pp.Detail, pp.Plan)
			}
		})
	}
}
