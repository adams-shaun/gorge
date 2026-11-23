package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Animate Abilities$ on the source itself (kgen animateSelfAbilities): Spawning
// Pool becomes a Skeleton with "{B}: Regenerate this creature" and Lavaclaw
// Reaches an Elemental with "{X}: This creature gets +X/+0" until end of
// turn, the abilities the override record grants the animated land.
var animateSelfAbilitiesDecks = [2][]cardset.Entry{
	{
		{Name: "Spawning Pool", Count: 8}, {Name: "Lavaclaw Reaches", Count: 8}, {Name: "Raging Goblin", Count: 6},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 14}, {Name: "Spawning Pool", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

// animateNoRepeatPolicy is castFirstPolicy that never activates the same
// ability twice in a row while another option exists. Lavaclaw Reaches'
// "{X}: +X/+0" with X=0 is a no-op a player can repeat forever holding
// priority; gorge's livelock detector rightly panics on that (kernel: a
// stack capacity stall), which is the player's loop, not a divergence.
func animateNoRepeatPolicy(seed uint64) Policy {
	inner := castFirstPolicy(seed)
	var last Canon
	var have bool
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if have && last.Kind() == ActActivate {
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a != last {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 && len(buf) < len(c.D.Options) {
				d := *c.D
				d.Options = buf
				c = &PickCtx{D: &d, G: c.G, Seq: c.Seq}
			}
		}
		pick := inner.Pick(c)
		if pick.Kind() < ActChoose { // a sub-choice (the X of the pump) is not a new action
			last, have = pick, true
		}
		return pick
	})
}

func TestAnimateAbilitiesDurationLockstep(t *testing.T) {
	t.Parallel()
	lockstepRunPolicy(t, "animateselfabilities", animateSelfAbilitiesDecks, 80, true, nil, nil, animateNoRepeatPolicy)
}

// TestAnimateAbilitiesDurationLockstepMutation: with the override records'
// grants dropped, some seed diverges (gorge offers the granted ability).
func TestAnimateAbilitiesDurationLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRunPolicy(t, "animateselfabilities", animateSelfAbilitiesDecks, 80, false, func(tbl *kop.Table) {
		ovs := append([]kop.Override(nil), tbl.Overrides...)
		for i := range ovs {
			ovs[i].Grant = [2]uint8{}
		}
		tbl.Overrides = ovs
	}, nil, animateNoRepeatPolicy)
	if n == 0 {
		t.Fatal("no seed diverged with the animated lands' granted abilities dropped")
	}
}
