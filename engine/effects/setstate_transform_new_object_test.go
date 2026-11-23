package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTransformSelfAfterReturnDoesNothing pins CR 400.7 / CR 701.28f for a
// transform ability naming its own source (Archangel Avacyn's delayed "transform
// Archangel Avacyn at the beginning of the next upkeep"): the ability was
// stamped with the source's incarnation when it was registered, and the card
// has since left the battlefield and come back. The permanent now on the
// battlefield is a new object, so the ability resolves without flipping it. The
// control resolves the same ability with an unchanged incarnation and must flip.
func TestTransformSelfAfterReturnDoesNothing(t *testing.T) {
	run := func(returned bool) int {
		h := newHost(t, 2)
		o := h.g.AddObject(transformingDFC(t), 0)
		o.Zone = state.ZBattlefield
		h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), o.ID))
		stamp := o.Incarnation
		if returned {
			o.Incarnation++
		}
		ab := h.g.AddObject(nil, 0)
		ab.Source = o.ID
		ab.PushSourceStamped, ab.PushSourceIncarnation = true, stamp
		// The delayed trigger was pushed in the next upkeep, while Avacyn
		// waited in her owner's hand: PushSourceOnBattlefield is false and
		// the stamp is the incarnation she had when it was registered.
		ab.StackKind, ab.StackKindKnown = state.StackKindTriggered, true
		if got := SourceIsNewObject(h.g, ab); got != returned {
			t.Fatalf("precondition: SourceIsNewObject = %v, want %v", got, returned)
		}
		Resolve(h, &Ctx{Source: o.ID, ResolvingObj: ab.ID, Controller: 0, Targets: []state.Target{{Obj: o.ID}}},
			sa(t, "DB$ SetState | Defined$ Self | Mode$ Transform"))
		return len(flipFaceEvents(h))
	}
	if n := run(false); n != 1 {
		t.Fatalf("control: FlipFace events = %d, want 1 for the same incarnation", n)
	}
	if n := run(true); n != 0 {
		t.Fatalf("FlipFace events = %d, want none for a returned source (new object)", n)
	}
}
