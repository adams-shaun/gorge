package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// Araumi of the Dead Tide's lockstep (dirac/kgen/lower_exilex.go's ability
// arm and lower_effect.go's Encore grant arm; kernel cost_exilex.go,
// kw_encoregrant.go): the pump is cast, survives a turn, and is activated on
// a graveyard creature card — gorge asks the target, then the exile pick of
// the fixed one-card cost (no X: pay.FixLifeXCost), and the pump
// grants the derived Encore keyword with no activation of it offered. The
// grant itself is unobservable (a graveyard card's derived keywords are not
// projected and nothing is offered), so the lockstep's observable surface is
// the cost: the exile pick. The mutation test drops the exile
// part and expects a divergence on exactly that.
//
// The deck is vanilla 2/2s that trade in combat and die, so a creature card
// reaches the graveyard for the pump to target.
func karaumiDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Araumi of the Dead Tide", Count: 4},
		{Name: "Accursed Marauder", Count: 16},
		{Name: "Swamp", Count: 12},
		{Name: "Island", Count: 8},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// karaumiPolicy casts, activates and plays lands whenever it can (so Araumi
// enters play, survives its summoning-sick turn and activates), else random;
// it counts, from gorge's options, the battlefield pump activations.
func karaumiPolicy(st *map[string]int) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		counted := map[int]bool{} // canonical decision counted this game
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority && !counted[c.Seq] {
				for _, a := range c.D.Options {
					if a.Kind() != ActActivate {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok || o.Kind != "ability" {
						continue
					}
					ob := c.G.E.G.Obj(o.Obj)
					if ob == nil || ob.Face() == nil || ob.Zone != state.ZBattlefield || ob.Face().Name != "Araumi of the Dead Tide" {
						continue
					}
					if *st == nil {
						*st = map[string]int{}
					}
					(*st)["acts"]++
					counted[c.Seq] = true
					break
				}
			}
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) == 0 || len(buf) == len(c.D.Options) {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		})
	}
}

func TestAraumiLockstep(t *testing.T) {
	t.Parallel()
	st := new(map[string]int)
	lockstepRunPolicy(t, "mbc", karaumiDecks(), 60, true, nil, nil, karaumiPolicy(st))
	if (*st)["acts"] == 0 {
		t.Fatal("Araumi's pump was never activated in the games; the lockstep proves nothing")
	}
	t.Logf("Araumi pump activations: %d", (*st)["acts"])
}

// TestAraumiLockstepMutationExile: with the pump's ExileFromGrave<X/Card>
// part dropped, some seed diverges (gorge asks the exile pick; the kernel
// would not).
func TestAraumiLockstepMutationExile(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "mbc", karaumiDecks(), 60, false, karaumiDropExile)
	if n == 0 {
		t.Fatal("no seed diverged with the pump's graveyard-exile part dropped")
	}
}

// karaumiDropExile clears kop.CostExileGrave from every ability cost (the
// deck's only carrier is Araumi's pump; the vanilla Marauders and the basic
// lands carry none).
func karaumiDropExile(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			abs[j].Cost.Flags &^= kop.CostExileGrave
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
}
