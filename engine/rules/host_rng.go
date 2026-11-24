package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/state"
)

// host_rng.go is the engine's implementation of effects.HostRNG, the
// randomness role of effects.Host (rules-engine refactor spec W1d): the
// engine's seeded RNG and the library shuffle that consumes it.

// Rand draws from the engine's seeded generator, except while the oracle
// runner is replaying a scenario with recorded die results: then
// dieRollOverride supplies the taped natural value for the matching die
// (CR 706.2's replay rule) and the generator is left untouched. A nil
// override -- every live game and every replay -- is exactly the old draw.
func (e *Engine) Rand(n int) int {
	if e.dieRollOverride != nil {
		if v, ok := e.dieRollOverride(n); ok {
			return v
		}
	}
	return e.rng.IntN(n)
}

// ShuffleLibrary is the single library-shuffle path used by rules and effects.
// State changes only when the caller emits the resulting Shuffle event.
func (e *Engine) ShuffleLibrary(player state.PlayerID, order []state.ObjID) []state.ObjID {
	out := append([]state.ObjID(nil), order...)
	s := e.rng.chance
	if s == nil || s.planner == nil {
		e.rng.Shuffle(out)
		return out
	}
	ordinal := s.shuffleOrdinals[player]
	s.shuffleOrdinals[player] = ordinal + 1
	ctx := ShuffleContext{Player: player, Ordinal: ordinal, Library: e.shuffleCards(out), Hand: e.shuffleCards(e.G.Zone(state.ZHand, player))}
	desired, err := s.planner(ctx)
	if err != nil {
		s.fail(fmt.Errorf("hypothetical shuffle planner: %w", err))
	}
	if desired == nil {
		e.rng.Shuffle(out)
		return out
	}
	if err := e.rng.forcePermutation(out, desired); err != nil {
		s.fail(err)
	}
	return out
}
