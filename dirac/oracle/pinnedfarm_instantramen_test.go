package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// A farm game (std-fin lockstep, explore bots) in which Instant Ramen is cast and
// sacrificed for life while a Saga chapter of Phoenix, Warden of Fire waits on the
// stack, and Stiltzkin, Moogle Merchant donates Phoenix to its opponent before the
// chapter resolves. The chapter's damage is dealt by Phoenix, so its lifelink gain
// goes to Phoenix's controller when it deals the damage, the damaged opponent, not
// to the chapter ability's controller (CR 702.15b). The decks are the farm game's,
// in the decklist order that deals its recorded opening hands and libraries at its
// seed.
func TestPinnedFarmInstantRamen(t *testing.T) {
	pinnedGames(t, "std-fin", Config{GorgeAutopaySurface: true},
		activated("Instant Ramen"), activated("Stiltzkin, Moogle Merchant"), lifelinkHitsItsController("Phoenix, Warden of Fire"))
}

// lifelinkHitsItsController is the condition that the named lifelink source dealt
// damage to the player controlling it, so the gain and the damage land on one player.
func lifelinkHitsItsController(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		evs := ref.E.L.Events
		for i := 1; i < len(evs); i++ {
			ev, hit := &evs[i], &evs[i-1]
			if ev.Kind != events.DamageProvenance || hit.Kind != events.Damage || hit.Obj != 0 {
				continue
			}
			o := ref.E.G.Obj(ev.Obj)
			if o != nil && o.Face() != nil && o.Face().Name == name && o.Controller == hit.Player {
				return true
			}
		}
		return false
	}
}
