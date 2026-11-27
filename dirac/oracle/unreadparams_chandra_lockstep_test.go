package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Chandra, Awakened Inferno's +2 is an Effect owned by each opponent
// (EffectOwner$ Player.Opponent) whose Phase trigger body is
// `DB$ DealDamage | Defined$ TriggeredPlayer | NumDmg$ 1`. gorge's
// Effect-registered Phase trigger now binds the player whose step it is, so
// the emblem damages its controller; kgen lowers the owner as
// CreateEmblem obj=EachOpp. The policy counts the emblem's 1 damage to a
// player (combat in this deck deals 2), proving the lowered line fired.
var chandraEmblemDeck = []cardset.Entry{
	{Name: "Chandra, Awakened Inferno", Count: 8}, {Name: "Grizzly Bears", Count: 5},
	{Name: "Savannah Lions", Count: 5}, {Name: "Mountain", Count: 22},
}

func TestUnreadParamsChandraEmblemLockstep(t *testing.T) {
	t.Parallel()
	d := chandraEmblemDeck
	hits := 0
	mk := func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		var last *state.Game
		scanned := 0
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				g := c.G.E.G
				if g != last {
					last, scanned = g, 0
				}
				evs := c.G.E.L.Events
				for _, e := range evs[scanned:] {
					if e.Kind == events.Damage && e.Obj == 0 && e.Amount == 1 {
						hits++
					}
				}
				scanned = len(evs)
			}
			return base.Pick(c)
		})
	}
	cedhPodRunWith(t, "chandraemblem", [][]cardset.Entry{d, d}, []string{"Chandra, Awakened Inferno"}, 40, nil, mk)
	if hits == 0 {
		t.Fatal("no emblem's upkeep trigger dealt 1 damage: the Phase TriggeredPlayer was never reached")
	}
	t.Logf("%d emblem upkeep damage events", hits)
}
