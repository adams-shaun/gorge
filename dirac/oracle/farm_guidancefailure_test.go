package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPinnedFarmGuidanceFailure replays the std-trk farm games that diverged
// with Guidance Failure implicated (TRK-s275 xe:RAND:3:7 turn 12, TRK-s250
// xe:RAND:2:5 turn 14). The Guidance Failure had been exiled by Reckless Impulse
// and was castable on its owner's next turn: a {3}{U} instant whose cost drops
// by {1} only for an attacking target, so gorge's planner leaves it to the
// potential-play lowering (a target-dependent cost). The owner also had a
// Starfleet Academy, whose {5},{T} filter makes one mana per creature type among
// their non-Borg creatures, here none. gorge offered the cast, paid by the other
// lands; dirac charged that filter's {5} fee off the same lands and offered
// nothing (CR 605.3a: a filter that makes no mana adds nothing to what the
// printed sources pay). The decks are the farm games', in the decklist order the
// farm's lockstep dealt them (sorted by name); the actions are the explore
// policy's up to a tail past the divergence. The conditions are the state the
// offer needs, held at some decision: the card in exile beside an untapped
// Starfleet Academy with four other untapped lands and no creature of its owner's
// (the filter makes nothing), and a creature of the opponent's to target.
func TestPinnedFarmGuidanceFailure(t *testing.T) {
	t.Parallel()
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	pinnedGamesRun(t, "std-trk", rc, 0, impulsedCastableWithIdleFilter)
}

// impulsedCastableWithIdleFilter: a seat has Guidance Failure in exile, an
// untapped Starfleet Academy, four other untapped lands and no creature, and
// the other seat has a creature to target.
func impulsedCastableWithIdleFilter(ref *GorgeSide) bool {
	g := ref.E.G
	for p := state.PlayerID(0); p < 2; p++ {
		inExile := false
		for _, id := range g.Zone(state.ZExile, p) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Guidance Failure" {
				inExile = true
			}
		}
		if !inExile {
			continue
		}
		academy, lands, creatures := false, 0, 0
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch {
			case ref.E.IsCreature(id):
				creatures++
			case o.Tapped:
			case o.Face().Name == "Starfleet Academy":
				academy = true
			case ref.E.IsLand(id):
				lands++
			}
		}
		targets := 0
		for _, id := range g.Zone(state.ZBattlefield, 1-p) {
			if ref.E.IsCreature(id) {
				targets++
			}
		}
		if academy && lands >= 4 && creatures == 0 && targets > 0 {
			return true
		}
	}
	return false
}
