package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The rad-counter lockstep (kgen api_life.go apiRadiation, kernel
// act_radiation.go): Feral Ghoul gives each opponent rad counters equal to
// its last power when it dies, Contaminated Drink gives its caster half X,
// Nuclear Fallout and Megaton's Fate every player X / four; Shock, the
// Fallout's -X/-X and the Fate's 8 damage kill the Ghouls. At the next
// precombat main of a player with rad counters, the source-less drain mills
// that many cards, and each nonland card milled costs 1 life and a rad
// counter. The projection compares every seat's rad count at every decision,
// so the per-card removal is checked, not inferred. Thrummingbird
// proliferates a player's rad counters. A Ghoul the Fallout kills dies with
// negative power; CR 107.1b treats the negative amount as zero, so both
// sides give nothing and every seat's rad count stays as it was.
func kradDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Feral Ghoul", Count: 4},
		{Name: "Contaminated Drink", Count: 3},
		{Name: "Nuclear Fallout", Count: 2},
		{Name: "Megaton's Fate", Count: 1},
		{Name: "Thrummingbird", Count: 3},
		{Name: "Walking Corpse", Count: 3},
		{Name: "Goblin Piker", Count: 2},
		{Name: "Shock", Count: 3},
		{Name: "Island", Count: 6},
		{Name: "Swamp", Count: 8},
		{Name: "Mountain", Count: 5},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// kradProbe counts, on gorge's side, the stack abilities the games played:
// Feral Ghoul's dies trigger (Face.Triggers[1]) and the source-less drain.
type kradProbe struct {
	ghoulRad, drain int
	radSeen         bool // some seat held a rad counter at a decision
}

func (p *kradProbe) observe(gs *GorgeSide) {
	g := gs.E.G
	for s := range g.Players {
		if g.Players[s].Counter("RAD") > 0 {
			p.radSeen = true
		}
	}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil || o.Ability == nil || o.Card != nil {
			continue
		}
		if o.Source == 0 && o.Ability.API == "RadiationDrain" {
			p.drain++
			continue
		}
		if src := g.Obj(o.Source); src != nil && src.Face() != nil && src.Face().Name == "Feral Ghoul" &&
			abilityIndex(src.Face(), o.Ability) == 0x81 {
			p.ghoulRad++
		}
	}
}

func TestRadiationLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := &dirac.RadiationStats
	gained, drains, milled, nonland := st.Gained.Load(), st.Drains.Load(), st.Milled.Load(), st.Nonland.Load()
	probe := &kradProbe{}
	policy := func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				probe.observe(c.G)
			}
			return base.Pick(c)
		})
	}
	seeds := monarchSeeds(120)
	if div := lockstepRunPolicy(t, "radiation", kradDecks(), seeds, true, nil, nil, policy); div != 0 {
		t.Errorf("%d of %d seeds diverged", div, seeds)
	}
	t.Logf("kernel: gained %d, drains %d, milled %d, nonland %d; gorge stack: Feral Ghoul rad %d, drain %d",
		st.Gained.Load()-gained, st.Drains.Load()-drains, st.Milled.Load()-milled, st.Nonland.Load()-nonland,
		probe.ghoulRad, probe.drain)
	for _, c := range []struct {
		n    int64
		what string
	}{
		{st.Gained.Load() - gained, "players given rad counters"},
		{st.Drains.Load() - drains, "drain triggers queued"},
		{st.Nonland.Load() - nonland, "nonland cards drained (life and a counter each)"},
		{st.Milled.Load() - milled - (st.Nonland.Load() - nonland), "lands milled by a drain (no life, no counter)"},
		{int64(probe.ghoulRad), "Feral Ghoul rad triggers on gorge's stack"},
		{int64(probe.drain), "drain triggers on gorge's stack"},
	} {
		if c.n <= 0 {
			t.Errorf("%s: none", c.what)
		}
	}
	if !probe.radSeen {
		t.Error("no seat ever held a rad counter at a decision")
	}
}
