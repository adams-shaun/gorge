package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// madnessAskOverVividCreek discards Fiery Temper (madness {R}) with a lone
// untapped Vivid Creek holding charge CHARGE counters and resolves the madness
// trigger up to its cast-or-not ask.
func madnessAskOverVividCreek(t *testing.T, seed uint64, charge int32) (*Engine, *decision.Decision, state.ObjID) {
	t.Helper()
	e, _, _ := altCostEngine(t, seed, []string{"Fiery Temper", "Vivid Creek"}, nil, nil)
	creek := moveToBattlefieldByName(t, e, 0, "Vivid Creek")
	// The move may or may not run the land's enters-tapped-with-counters
	// replacement: set the board this test reads either way.
	if e.G.Obj(creek).Tapped {
		e.emit(events.Event{Kind: events.Untap, Obj: creek})
	}
	if delta := charge - e.G.Obj(creek).Counter("CHARGE"); delta != 0 {
		e.emit(events.Event{Kind: events.CounterChange, Obj: creek, Counter: "CHARGE", Amount: delta})
	}
	if o := e.G.Obj(creek); o.Tapped || o.Counter("CHARGE") != charge || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: Vivid Creek tapped=%v charge=%d pool=%d, want untapped, %d, empty",
			o.Tapped, o.Counter("CHARGE"), e.G.Players[0].Pool.Total(), charge)
	}
	id := findCardObj(t, e, 0, "Fiery Temper", state.ZHand)
	e.pending = nil
	e.emit(events.Discard(id, 0))
	e.priorityRound()
	passOnceP(t, e)
	passOnceP(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional || d.Source != id {
		t.Fatalf("madness resolution choice: %+v", d)
	}
	return e, d, id
}

// TestMadnessOfferedFromCounterCostSource: CR 702.35a with CR 601.2g -- the
// madness ask offers the cast when only a counter-cost mana ability (Vivid
// Creek's "{T}, remove a charge counter: one mana of any color") makes the
// owed colour. The planner's raw verdict is "insufficient" because its source
// census misses that ability, so it is no proof the cast cannot be paid.
func TestMadnessOfferedFromCounterCostSource(t *testing.T) {
	t.Parallel()
	t.Run("charge counters left", func(t *testing.T) {
		e, d, temper := madnessAskOverVividCreek(t, 941, 2)
		raw := e.potentialModeCastPlan(0, decision.Option{Kind: "cast", Obj: temper, Mode: altMode(altMadness)})
		if raw.Plan != nil || raw.Reason != pay.ReasonInsufficient {
			t.Fatalf("precondition: the planner's raw verdict = %q (plan %v), want insufficient", raw.Reason, raw.Plan != nil)
		}
		if optionIndexForKind(d, "yes") < 0 || optionIndexForKind(d, "no") < 0 || len(d.Options) != 2 {
			t.Fatalf("madness ask options = %+v, want yes and no", d.Options)
		}
	})
	// Without a counter the second ability cannot be activated (CR 602.2b) and
	// the {U} ability cannot pay {R}: the proof holds and only "no" is offered.
	t.Run("no charge counter", func(t *testing.T) {
		_, d, _ := madnessAskOverVividCreek(t, 942, 0)
		if len(d.Options) != 1 || d.Options[0].Kind != "no" {
			t.Fatalf("madness ask options = %+v, want only no", d.Options)
		}
	})
}
