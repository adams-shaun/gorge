package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// carrierLifetimeBoard builds two Effect activations of one source: delayed
// registrations stamped at 9 and 19 and their continuous halves stamped just
// after each (an Effect registers Triggers$ before StaticAbilities$), plus a
// printed static of the same source that neither self-exile may touch.
func carrierLifetimeBoard(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := newSeats(t, 2)
	e.pending = nil
	src := onBoard(t, e, 0, effectTriggerSelfExileSrc)
	e.continuous = append(e.continuous[:0],
		state.ContinuousEffect{Source: src, FromEffect: true, Timestamp: 10, Affects: "Card.Self", AddKeywords: []string{"Flying"}},
		state.ContinuousEffect{Source: src, FromEffect: true, Timestamp: 20, Affects: "Card.Self", AddKeywords: []string{"Shroud"}},
		state.ContinuousEffect{Source: src, Timestamp: 30, Affects: "Card.Self", AddKeywords: []string{"Trample"}},
	)
	e.G.Delayed = append(e.G.Delayed[:0],
		state.DelayedTrigger{ID: 0, Source: src, EffectRepeat: true, RegistrationClock: 9},
		state.DelayedTrigger{ID: 1, Source: src, EffectRepeat: true, RegistrationClock: 19},
	)
	return e, src
}

func carrierHasContinuous(e *Engine, src state.ObjID, kw string) bool {
	for _, ce := range e.continuous {
		if ce.Source == src && len(ce.AddKeywords) == 1 && ce.AddKeywords[0] == kw {
			return true
		}
	}
	return false
}

// TestEffectCarrierSelfExileSparesAnotherActivation pins the carrier frame's
// registration lifetime: an Effect's own trigger body self-exile ends exactly
// that activation, so a second activation of the same source keeps its
// trigger and continuous half and still fires later (Ugin, the Ineffable's
// second [+1] while the first Spirit is alive). Before the carrier bound the
// ender dropped every Effect repeat registration from the source.
func TestEffectCarrierSelfExileSparesAnotherActivation(t *testing.T) {
	t.Parallel()

	t.Run("newer carrier spares the older activation", func(t *testing.T) {
		e, src := carrierLifetimeBoard(t)
		e.EndEffectSource(src, 19, true)
		if len(e.G.Delayed) != 1 || e.G.Delayed[0].ID != 0 || e.G.Delayed[0].RegistrationClock != 9 {
			t.Fatalf("after the newer carrier's self-exile, delayed = %+v, want only the older registration", e.G.Delayed)
		}
		if !carrierHasContinuous(e, src, "Flying") {
			t.Fatalf("the older activation's continuous half was ended: %+v", e.continuous)
		}
		if carrierHasContinuous(e, src, "Shroud") {
			t.Fatalf("the newer carrier's own continuous half survived: %+v", e.continuous)
		}
		if !carrierHasContinuous(e, src, "Trample") {
			t.Fatalf("the source's printed static was ended: %+v", e.continuous)
		}
	})

	t.Run("older carrier spares the newer activation", func(t *testing.T) {
		e, src := carrierLifetimeBoard(t)
		e.EndEffectSource(src, 9, true)
		if len(e.G.Delayed) != 1 || e.G.Delayed[0].ID != 1 || e.G.Delayed[0].RegistrationClock != 19 {
			t.Fatalf("after the older carrier's self-exile, delayed = %+v, want only the newer registration", e.G.Delayed)
		}
		if carrierHasContinuous(e, src, "Flying") {
			t.Fatalf("the older carrier's own continuous half survived: %+v", e.continuous)
		}
		if !carrierHasContinuous(e, src, "Shroud") {
			t.Fatalf("the newer activation's continuous half was ended: %+v", e.continuous)
		}
		if !carrierHasContinuous(e, src, "Trample") {
			t.Fatalf("the source's printed static was ended: %+v", e.continuous)
		}
	})
}
