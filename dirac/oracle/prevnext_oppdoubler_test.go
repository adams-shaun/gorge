package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A depleting prevent-the-next-N shield (Benevolent Ancestor) on a player meets burn from the opponent while the shield's controller holds
// a Fire Servant. Fire Servant doubles only the damage of its own controller's
// red instants and sorceries (CR 614.1), so the opponent's burn has a single
// candidate replacement and gorge asks no CR 616.1 order: dirac must apply the
// shield on its own, not stall StallPrevNext on the unfiltered doubler scan.
var kprevnextOppDoublerDecks = [2][]cardset.Entry{
	{
		{Name: "Fire Servant", Count: 8}, {Name: "Benevolent Ancestor", Count: 8},
		{Name: "Plains", Count: 12}, {Name: "Mountain", Count: 12},
	},
	{
		{Name: "Lightning Bolt", Count: 10}, {Name: "Lava Dart", Count: 10}, {Name: "Mountain", Count: 20},
	},
}

// pnxOppDoublerPolicy is pnxPolicy except seat 0 acts whatever the stack holds:
// it casts or activates what it can (Fire Servant, a shield on itself or on the
// player the burn on the stack targets) before seat 1's burn resolves.
type pnxOppDoublerPolicy struct{ pnxPolicy }

func (p pnxOppDoublerPolicy) Pick(c *PickCtx) Canon {
	if d := c.D; d.Kind == DPriority && d.Player == 0 {
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
			return a
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast || a.Kind() == ActActivate }); ok {
			return a
		}
	}
	return p.pnxPolicy.Pick(c)
}

func TestPrevNextOpponentDoublerLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(300)
	if v := os.Getenv("KPREVNEXTOD_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	lockstepRunPolicy(t, "mbc", kprevnextOppDoublerDecks, seeds, true, nil, nil, func(seed uint64) Policy {
		return pnxOppDoublerPolicy{pnxPolicy{RandomPolicy(seed)}}
	})
}
