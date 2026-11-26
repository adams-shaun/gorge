package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// An until-end-of-turn Effect with RememberObjects$ Self whose trigger body
// reads the source back as Defined$ Remembered, in lockstep with gorge:
// Mountain Titan ("until end of turn, whenever you cast a black spell, put a
// +1/+1 counter on Mountain Titan") and Dovin, Grand Arbiter's +1 ("whenever
// a creature you control deals combat damage to a player, put a loyalty
// counter on Dovin"). gorge's effectRemembered stores the source by id; kgen
// lowers the body's Remembered to kop.RefEmblemSource (emblemSelfRemember),
// the emblem's source as it was when the trigger was queued.

var mbcSelfRemDecks = [2][]cardset.Entry{
	{
		{Name: "Mountain Titan", Count: 10}, {Name: "Walking Corpse", Count: 6}, {Name: "Mind Rot", Count: 3},
		{Name: "Doom Blade", Count: 3}, {Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Dovin, Grand Arbiter", Count: 10}, {Name: "Savannah Lions", Count: 6}, {Name: "Elite Vanguard", Count: 4},
		{Name: "Wind Drake", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 8},
	},
}

// mbcSelfRemStats counts, over all games, the Effect registrations gorge
// held at a pick and the source permanents seen carrying the counters the
// emblem's body puts on them (ids repeat from game to game, so each policy
// instance, one per seed, remembers its own).
type mbcSelfRemStats struct {
	titanRegs, dovinRegs int // Effect trigger registrations
	titanGrown, dovinUp  int // a Mountain Titan with a +1/+1 counter; a Dovin above 4 loyalty
}

func mbcSelfRemProbe(base Policy, st *mbcSelfRemStats) Policy {
	seen := map[uint64]bool{}
	grown := map[state.ObjID]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			for _, dt := range g.Delayed {
				o := g.Obj(dt.Source)
				if o == nil || o.Face() == nil || seen[uint64(dt.ID)] {
					continue
				}
				switch {
				case dt.EventMode == "SpellCast" && o.Face().Name == "Mountain Titan":
					st.titanRegs++
				case dt.EventMode == "DamageDone" && o.Face().Name == "Dovin, Grand Arbiter":
					st.dovinRegs++
				default:
					continue
				}
				seen[uint64(dt.ID)] = true
			}
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range g.Zone(state.ZBattlefield, p) {
					ob := g.Obj(id)
					if ob == nil || ob.Face() == nil || grown[id] {
						continue
					}
					switch {
					case ob.Face().Name == "Mountain Titan" && ob.Counter("P1P1") > 0:
						st.titanGrown++
					case ob.Face().Name == "Dovin, Grand Arbiter" && ob.Counter("LOYALTY") > 4:
						st.dovinUp++
					default:
						continue
					}
					grown[id] = true
				}
			}
		}
		return base.Pick(c)
	})
}

func TestMBCEffectSelfRememberLowers(t *testing.T) {
	t.Parallel()
	var st mbcSelfRemStats
	pol := func(seed uint64) Policy { return mbcSelfRemProbe(castFirstPolicy(seed), &st) }
	const seeds = 200
	if div := lockstepRunPolicy(t, "mbcselfrem", mbcSelfRemDecks, seeds, true, nil, nil, pol); div != 0 {
		return // each divergence is already a test error
	}
	t.Logf("%d seeds: %+v", seeds, st)
	if st.titanRegs < 20 || st.dovinRegs < 20 || st.titanGrown < 5 || st.dovinUp < 5 {
		t.Fatalf("%d seeds reached %+v, want at least 20 registrations and 5 grown sources of each: the pool does not exercise it", seeds, st)
	}
}

// TestMBCEffectSelfRememberMutation: a body that reads RefSelf (the emblem
// entry's command-zone copy, which names nothing) puts no counter, where
// gorge puts one on the Effect's source.
func TestMBCEffectSelfRememberMutation(t *testing.T) {
	t.Parallel()
	self := func(tbl *kop.Table) {
		tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
		for i := range tbl.Cards {
			abs := append([]kop.Ability(nil), tbl.Cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for n := range prog {
					if prog[n].Obj == kop.RefEmblemSource {
						prog[n].Obj = kop.RefSelf
					}
				}
				abs[j].Prog = prog
			}
			tbl.Cards[i].Abilities = abs
		}
	}
	var st mbcSelfRemStats
	pol := func(seed uint64) Policy { return mbcSelfRemProbe(castFirstPolicy(seed), &st) }
	if div := lockstepRunPolicy(t, "mbcselfrem", mbcSelfRemDecks, 200, false, self, nil, pol); div == 0 {
		t.Fatal("an emblem body over RefSelf played 200 seeds with no divergence")
	}
}
