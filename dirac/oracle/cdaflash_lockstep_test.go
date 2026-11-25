package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The conditional CDA flash lockstep: a card's own
// `Mode$ Continuous | CharacteristicDefining$ True | AddKeyword$ Flash |
// IsPresent$ <type>.YouCtrl` static (kgen's lower_cdaflash.go) lets it be
// cast at instant speed from hand while the gate holds for the caster.
// Colossal Rattlewurm (a Desert) and Crashing Tide (a Merfolk) are the
// carriers; each deck holds its gate's enabler.

func cdaFlashDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{ // A: Colossal Rattlewurm and Deserts
			{Name: "Colossal Rattlewurm", Count: 3}, {Name: "Desert", Count: 4},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Giant Spider", Count: 4}, {Name: "Giant Growth", Count: 4},
			{Name: "Fog", Count: 2}, {Name: "Forest", Count: 15},
		},
		{ // B: Crashing Tide and Merfolk
			{Name: "Crashing Tide", Count: 4}, {Name: "Merfolk Looter", Count: 4},
			{Name: "Cloud Sprite", Count: 4}, {Name: "Cancel", Count: 4},
			{Name: "Unsummon", Count: 4}, {Name: "Divination", Count: 4},
			{Name: "Island", Count: 16},
		},
	}
}

type cdaFlashStats struct {
	casts   int // off-sorcery-timing casts of a CDA flash carrier
	blocked int // priority windows where a carrier in hand was not offered, its gate failing off-timing
	byName  map[string]int
}

// cdaFlashProbe wraps a seeded random policy biased to take an off-timing
// carrier cast when one is offered, and counts the casts and the windows
// where a carrier was withheld by its gate.
type cdaFlashProbe struct {
	st   *cdaFlashStats
	base Policy
	r    *rand.Rand
}

// cdaFlashGate is the carrier's gate for player p, read from gorge's state
// independently of the kernel.
func cdaFlashGate(g *state.Game, p state.PlayerID, name string) (carrier, holds bool) {
	count := func(z state.Zone, ty string) int {
		n := 0
		for _, id := range g.Zone(z, p) {
			if o := g.Obj(id); o != nil && o.Face() != nil && hasFaceType(o.Face(), ty) {
				n++
			}
		}
		return n
	}
	switch name {
	case "Colossal Rattlewurm":
		return true, count(state.ZBattlefield, "Desert") >= 1
	case "Crashing Tide":
		return true, count(state.ZBattlefield, "Merfolk") >= 1
	}
	return false, false
}

func (p *cdaFlashProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	d := c.G.Pending()
	if d == nil || d.Kind != "priority" {
		return a
	}
	g := c.G.E.G
	off := !(g.Step.IsMain() && g.Active == g.Priority && len(g.Stack) == 0)
	if !off {
		return a
	}
	var offered []Canon                 // option order
	offeredID := map[state.ObjID]bool{} // membership only, never ranged
	for _, x := range c.D.Options {
		if x.Kind() != ActCast {
			continue
		}
		if o, ok := c.G.GorgeOption(x); ok {
			if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil {
				if carrier, _ := cdaFlashGate(g, g.Priority, ob.Face().Name); carrier {
					offered = append(offered, x)
					offeredID[o.Obj] = true
				}
			}
		}
	}
	for _, id := range g.Zone(state.ZHand, g.Priority) {
		ob := g.Obj(id)
		if ob == nil || ob.Face() == nil {
			continue
		}
		if carrier, holds := cdaFlashGate(g, g.Priority, ob.Face().Name); carrier && !holds {
			if !offeredID[id] {
				p.st.blocked++
			}
		}
	}
	if len(offered) > 0 && p.r.IntN(4) != 0 {
		a = offered[p.r.IntN(len(offered))]
	}
	if a.Kind() == ActCast {
		for _, x := range offered {
			if x == a {
				p.st.casts++
				if o, ok := c.G.GorgeOption(a); ok {
					if p.st.byName == nil {
						p.st.byName = map[string]int{}
					}
					p.st.byName[g.Obj(o.Obj).Face().Name]++
				}
			}
		}
	}
	return a
}

// cdaFlashPolicy runs the probe: it takes an offered carrier cast off-timing
// most of the time, else picks like the seeded random policy.
func cdaFlashPolicy(st *cdaFlashStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		return PolicyFunc((&cdaFlashProbe{st: st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6364666c))}).pick)
	}
}

func TestCDAFlashLockstep(t *testing.T) {
	t.Parallel()
	var st cdaFlashStats
	lockstepRunPolicy(t, "cdaflash", cdaFlashDecks(), 60, true, nil, nil, cdaFlashPolicy(&st))
	for _, name := range []string{"Colossal Rattlewurm", "Crashing Tide"} {
		if st.byName[name] < 1 {
			t.Errorf("%s: %d off-timing casts, want at least 1 (the probe proved nothing)", name, st.byName[name])
		}
	}
	if st.blocked < 1 {
		t.Errorf("%d windows with a carrier withheld by its gate, want at least 1", st.blocked)
	}
	t.Logf("CDA flash %+v", st)
}

// TestCDAFlashMutation: selfFlash reading false must make the lockstep
// diverge (gorge offers the carriers at instant speed, the kernel does not).
func TestCDAFlashMutation(t *testing.T) {
	dirac.DebugSelfFlashOff = true
	defer func() { dirac.DebugSelfFlashOff = false }()
	if n := lockstepRun(t, "cdaflash", cdaFlashDecks(), 64, false, nil); n == 0 {
		t.Fatal("DebugSelfFlashOff: no divergence in 64 seeds")
	}
}
