package oracle

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The granted mana alternative cost lockstep: Fist of Suns' "you may pay
// {W}{U}{B}{R}{G} rather than pay the mana cost for spells that you cast" and
// Rooftop Storm's "you may pay {0} rather than pay the mana cost for Zombie
// creature spells you cast" (gorge alternativeCosts' printed battlefield
// walk: one AltCostIndex cast per applicable grant, cast_altgrant.go). The
// oracle plays gorge's payment-planner surface, which skips AltCostIndex
// options (payment_plan.go), so a priced alternative is offered only while the
// floating pool pays it: the policy floats its lands' mana once a Fist is out
// (all five colours need five lands), and takes the alternative cast whenever
// it is offered. A Zombie cast at Rooftop Storm's {0} owes nothing from the
// pool. The expected answer at every decision is gorge's.
var kaltgrantDecks = [2][]cardset.Entry{
	{
		{Name: "Fist of Suns", Count: 6}, {Name: "Rooftop Storm", Count: 3}, {Name: "Walking Corpse", Count: 4}, {Name: "Blaze", Count: 2},
		{Name: "Channel the Suns", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Island", Count: 5},
		{Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 20}, {Name: "Forest", Count: 20},
	},
}

// altGrantStats are the preconditions measured from gorge's side: how many
// priority decisions listed a granted alternative cast (a "cast" option at
// AltCostIndex 1 or more, no mode) of a card Rooftop Storm's grant admits and
// of one only Fist of Suns' does, how many {X} spell (Blaze) offers, how many listed both grants of one Zombie
// (AltCostIndex 2), and how many were taken.
type altGrantStats struct {
	fistOffers, stormOffers, xOffers, bothOffers, taken atomic.Int64
}

// altGrantPolicy casts Fist of Suns, then Channel the Suns ({3}{G}: add
// {W}{U}{B}{R}{G}, floating through the main phase, the only way a pool holds
// all five colours at the oracle's payment-planner surface), takes any granted
// alternative cast offered, the first and the last by turns, and plays
// everything else at random.
func altGrantPolicy(st *altGrantStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		var n int
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G == nil || c.D.Kind != DPriority {
				return rnd.Pick(c)
			}
			var alt []Canon
			for _, a := range c.D.Options {
				if a.Kind() != ActCast || a.Mode()&0xF != 0 || a.Mode()>>4 < 1 {
					continue
				}
				alt = append(alt, a)
				switch {
				case a.Mode()>>4 == 2:
					st.bothOffers.Add(1)
				case refCardName(c.G, a.Src()) == "Blaze":
					st.xOffers.Add(1) // an {X}{R} spell: the grant prices it at X = 0 (CR 107.3b)
				case refCardName(c.G, a.Src()) == "Walking Corpse":
					st.stormOffers.Add(1)
				default:
					st.fistOffers.Add(1)
				}
			}
			if len(alt) > 0 {
				st.taken.Add(1)
				n++
				return alt[(len(alt)-1)*(n%2)]
			}
			for _, name := range []string{"Fist of Suns", "Channel the Suns"} {
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && a.Mode() == 0 && refCardName(c.G, a.Src()) == name && (name == "Fist of Suns" || fistOut(c.G, c.D.Player)) {
						return a
					}
				}
			}
			return rnd.Pick(c)
		})
	}
}

// fistOut reports whether player's battlefield holds a Fist of Suns.
func fistOut(g *GorgeSide, player uint8) bool {
	e := g.E.G
	for _, id := range e.Zone(state.ZBattlefield, state.PlayerID(player)) {
		if o := e.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Fist of Suns" {
			return true
		}
	}
	return false
}

func runAltGrant(t *testing.T, seeds uint64, mutate func(*kop.Table)) (*altGrantStats, []lockstepDivergence) {
	t.Helper()
	reg, set, gd := lockstepPool(t, "altcostgrant", kaltgrantDecks)
	st := &altGrantStats{}
	var div []lockstepDivergence
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		_, _, d := lockstepSeedsMode(t, reg, set, "altcostgrant", kaltgrantDecks, gd, lo, hi, lockstepQuiet, mutate, nil, altGrantPolicy(st))
		mu.Lock()
		div = append(div, d...)
		mu.Unlock()
	})
	return st, div
}

// TestAltCostGrantLockstep: the pool plays both grants, taken at gorge's
// AltCostIndex, with no divergence, and the policy reached each grant's offer.
func TestAltCostGrantLockstep(t *testing.T) {
	t.Parallel()
	st, div := runAltGrant(t, 300, nil)
	t.Logf("fist offers %d, storm offers %d, x-spell offers %d, both %d, taken %d", st.fistOffers.Load(), st.stormOffers.Load(), st.xOffers.Load(), st.bothOffers.Load(), st.taken.Load())
	if st.xOffers.Load() == 0 {
		t.Error("no priority decision listed Blaze's alternative cast: the pool never reaches an {X} spell cast through a grant")
	}
	if st.fistOffers.Load() == 0 {
		t.Error("no priority decision listed Fist of Suns' alternative cast: the pool never reaches it")
	}
	if st.stormOffers.Load() == 0 {
		t.Error("no priority decision listed Rooftop Storm's alternative cast: the pool never reaches it")
	}
	if st.bothOffers.Load() == 0 {
		t.Error("no Zombie was listed with both grants (AltCostIndex 2): the pool never reaches the second index")
	}
	if len(div) > 0 {
		t.Error(divergenceReport("altcostgrant", 300, div))
	}
}

// TestAltCostGrantMutation: with the grants dropped from the kernel's table
// some seed diverges, so the pool does exercise the offer.
func TestAltCostGrantMutation(t *testing.T) {
	t.Parallel()
	_, div := runAltGrant(t, 300, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				if a := &tbl.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost && a.Static.StFlags&kop.StAltGrant != 0 {
					a.Static.Kind = kop.StatNone
				}
			}
		}
	})
	if len(div) == 0 {
		t.Error("no seed diverged with the grants dropped: the pool never exercises them")
	}
}
