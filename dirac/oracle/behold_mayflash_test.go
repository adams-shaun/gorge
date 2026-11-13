package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The behold may-flash lockstep: Molten Exhale's "you may cast this spell as
// though it had flash if you behold a Dragon as an additional cost" (a
// MayFlashCost extra that is a one-Dragon behold, CR 702.8, 701.4a). gorge's
// hand walk offers the "mayflash" cast at instant timing only while its caster has a
// Dragon on the battlefield or another one in hand, and its beholdCostAsk
// poses the pick after the targets (a lone candidate taken unasked); at
// sorcery timing only the plain cast is offered, Dragon or none. The kernel
// (kw_mayflash.go, act_behold.go) must match it decision by decision.
//
// Sarkhan, Dragon Ascendant's trigger-cost behold is not here: gorge's
// triggered-cost window offers only "Do not pay" for a Behold part (see the
// ticket's report).
var kbmfDecks = [2][]cardset.Entry{
	{
		{Name: "Savage Ventmaw", Count: 4}, {Name: "Molten Exhale", Count: 6}, {Name: "Dragon Hatchling", Count: 3},
		{Name: "Furnace Whelp", Count: 3}, {Name: "Dragon Moose", Count: 2}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Forest", Count: 7}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Grand Warlord Radha", Count: 4}, {Name: "Molten Exhale", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Raging Goblin", Count: 6}, {Name: "Llanowar Elves", Count: 4}, {Name: "Forest", Count: 6},
		{Name: "Mountain", Count: 10},
	},
}

type kbmfStats struct {
	asked, unasked int // may-flash casts with two or more Dragon candidates / exactly one
	withheld       int // instant-timing priority with Molten Exhale in hand, {1}{R} floating, no Dragon: no may-flash offer
	plain          int // sorcery-timing plain casts of Molten Exhale with no Dragon to behold
}

// kbmfDragons counts p's behold candidates for Molten Exhale src: Dragons p
// controls and Dragon cards in p's hand.
func kbmfDragons(g *state.Game, p state.PlayerID, src state.ObjID) int {
	n := 0
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Controller == p && objectHasType(o, "Dragon") {
			n++
		}
	}
	for _, id := range g.Zone(state.ZHand, p) {
		if id != src && objectHasType(g.Obj(id), "Dragon") {
			n++
		}
	}
	return n
}

func kbmfExhaleInHand(g *state.Game, p state.PlayerID) state.ObjID {
	for _, id := range g.Zone(state.ZHand, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Molten Exhale" {
			return id
		}
	}
	return 0
}

// kbmfPolicy attacks with everything on most attack picks (Savage
// Ventmaw's and Grand Warlord Radha's attack triggers float the mana: gorge
// prices a may-flash cast only from the floating pool), takes the may-flash
// cast whenever it is offered, and otherwise casts what it can.
func kbmfPolicy(st *kbmfStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		r := rand.New(rand.NewPCG(seed, 0x6b626d66))
		var buf []Canon
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G == nil {
				return rnd.Pick(c)
			}
			if c.D.Kind == DAttackers {
				for _, x := range c.D.Options {
					if x.Kind() == ActAttack && r.IntN(5) != 0 {
						return x
					}
				}
				return rnd.Pick(c)
			}
			if c.D.Kind != DPriority {
				return rnd.Pick(c)
			}
			g := c.G.E.G
			p := state.PlayerID(c.D.Player)
			sorcery := g.Active == p && (g.Step == state.StepMain1 || g.Step == state.StepMain2) && len(g.Stack) == 0
			ex := kbmfExhaleInHand(g, p)
			var mf Canon
			hasMF := false
			buf = buf[:0]
			for _, x := range c.D.Options {
				o, ok := c.G.GorgeOption(x)
				if ok && o.Kind == "cast" && o.Mode == "mayflash" {
					mf, hasMF = x, true
				}
				if x.Kind() == ActCast || x.Kind() == ActPlayLand {
					buf = append(buf, x)
				}
			}
			if ex != 0 && !sorcery && !hasMF && kbmfDragons(g, p, ex) == 0 && g.Players[p].Pool.Total() >= 2 && g.Players[p].Pool[state.MR] >= 1 {
				st.withheld++
			}
			if hasMF {
				switch n := kbmfDragons(g, p, ex); {
				case n >= 2:
					st.asked++
				case n == 1:
					st.unasked++
				}
				return mf
			}
			if len(buf) == 0 || r.IntN(4) == 0 {
				return rnd.Pick(c)
			}
			d := *c.D
			d.Options = buf
			a := rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			if o, ok := c.G.GorgeOption(a); ok && o.Kind == "cast" && o.Mode == "" && o.Obj == ex && ex != 0 && kbmfDragons(g, p, ex) == 0 {
				st.plain++
			}
			return a
		})
	}
}

func TestBeholdTriggerMayFlashLockstep(t *testing.T) {
	t.Parallel()
	var st kbmfStats
	lockstepRunPolicy(t, "beholdmayflash", kbmfDecks, 300, true, nil, nil, kbmfPolicy(&st))
	t.Logf("may-flash casts: %d asked, %d unasked; %d withheld without a Dragon; %d plain casts without one", st.asked, st.unasked, st.withheld, st.plain)
	for _, c := range []struct {
		n    int
		what string
	}{
		{st.asked, "may-flash casts with the behold asked"},
		{st.unasked, "may-flash casts with a lone Dragon taken unasked"},
		{st.withheld, "instant-timing windows withholding the may-flash cast without a Dragon"},
		{st.plain, "sorcery-timing casts without a Dragon"},
	} {
		if c.n < 1 {
			t.Errorf("no %s", c.what)
		}
	}
}
