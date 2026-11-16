package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A mana source whose tap a trigger watches (Relic Bind's choose one) is
// deferred for the payment planner (payment_plan_interference.go names the
// watching object), but the activation it would fund is a legal play the
// payer completes in the ordinary manual window: the trigger the tap fires is
// pushed and resolves at the next priority round, after the activation
// (CR 405.2), so the offer gate may not withhold it. This is the offer half of
// the tail-08 pair-159 lockstep divergence: the kernel offered Memorial to
// Unity's {2}{G}, {T}, Sacrifice dig and gorge withheld it while p0's Relic
// Bind watched the p1 Talisman of Unity that covered the {2}.
func TestRelicBindDeferredSourceStillOffersDig(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := linkBoard(t, reg, []string{"Relic Bind"}, []string{"Memorial to Unity", "Island", "Forest", "Talisman of Unity"})
	talisman := findOnBoard(t, e, 1, "Talisman of Unity")
	memorial := findOnBoard(t, e, 1, "Memorial to Unity")
	relic := moveFromAnywhere(t, e, 0, "Relic Bind", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: relic, IDs: []state.ObjID{talisman}})
	// Memorial to Unity enters tapped (its ETB replacement); the divergence
	// window had it untapped, so untap it before the offer read.
	e.emit(events.Event{Kind: events.Untap, Obj: memorial})
	// Preconditions: the watch is live (the aura really is attached), the dig
	// source is untapped (it enters tapped), and the census covers {2}{G}
	// without the dig source's own mana (Island, Forest, Talisman).
	if e.G.Obj(relic).AttachedTo != talisman {
		t.Fatalf("precondition: Relic Bind not watching the Talisman (%d -> %d)", e.G.Obj(relic).AttachedTo, talisman)
	}
	if e.G.Obj(memorial).Tapped {
		t.Fatal("precondition: the dig source is tapped, the {T} cost is the thing under test")
	}
	if _, ok := findAbilityOptionIn(e.legalActions(1), memorial, 1); !ok {
		t.Fatal("Memorial to Unity's dig withheld while a Relic-Bind-watched Talisman covers the {2}{G}")
	}
}

// moveFromAnywhere moves seat p's card named name to to wherever it sits now
// (linkBoard parks a listed aura on the battlefield unattached, and a
// state-based action then buries it in the graveyard).
func moveFromAnywhere(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone) state.ObjID {
	t.Helper()
	for id := range e.G.Objs {
		if o := &e.G.Objs[id]; o != nil && o.Owner == p && o.Card != nil && o.Face() != nil && o.Face().Name == name {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: to})
			return o.ID
		}
	}
	t.Fatalf("card %q not owned by seat %d anywhere", name, p)
	return 0
}

// findAbilityOptionIn is findAbilityOption over a caller-built option list.
func findAbilityOptionIn(opts []decision.Option, id state.ObjID, idx int) (decision.Option, bool) {
	for _, o := range opts {
		if o.Kind == "ability" && o.Obj == id && o.Ability == idx {
			return o, true
		}
	}
	return decision.Option{}, false
}
