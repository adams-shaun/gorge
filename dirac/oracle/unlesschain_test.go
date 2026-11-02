package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorge binds a chained Counter link's own target before
// its UnlessCost$ gate, so the default payer (Swallowed by Leviathan: a Surveil
// head, then "counter the chosen spell unless its controller pays") names the
// COUNTERED spell's controller, not the resolving controller. The pool pits
// the counter against creature spells and burn; the policy leans toward
// countering an opponent's spell, so the unless ask lands on the opponent of
// the counter's controller. A kernel that asks the resolving controller
// diverges on the ask's player. Repulsive Mutation stays refused (its up-to
// link elects zero and gorge then asks the resolving controller; the kgen
// refusal in lowerCtx.unless).

var unlessChainDecks = [][2][]cardset.Entry{
	{ // M1: the two counters vs creatures and burn
		{
			{Name: "Swallowed by Leviathan", Count: 8},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Growth", Count: 4},
			{Name: "Island", Count: 11}, {Name: "Forest", Count: 11},
		},
		{
			{Name: "Raging Goblin", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Child of Night", Count: 4},
			{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Mountain", Count: 14}, {Name: "Swamp", Count: 6},
		},
	},
	{ // M2: counters on both sides
		{
			{Name: "Swallowed by Leviathan", Count: 8},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Growth", Count: 4},
			{Name: "Island", Count: 11}, {Name: "Forest", Count: 11},
		},
		{
			{Name: "Swallowed by Leviathan", Count: 8},
			{Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Raging Goblin", Count: 4},
			{Name: "Island", Count: 8}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 5},
		},
	},
}

type unlessChainStats struct {
	resolves    int // a Swallowed by Leviathan resolved
	foreignAsks int // a decision during one's resolution posed to the OTHER player (the unless ask)
	ownAsks     int // a decision during one's resolution posed to its controller
}

func unlessChainCounter(n string) bool { return n == "Swallowed by Leviathan" }

// unlessChainPolicy casts a counter when an opponent's spell is on the stack,
// otherwise leans toward casting at all.
func unlessChainPolicy(seed uint64, st *unlessChainStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6b666f6c3338)) // "kfol38"
	var buf []Canon
	evAt := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		evs := c.G.E.L.Events
		if len(evs) < evAt {
			evAt = 0
		}
		for j := evAt; j < len(evs); j++ {
			if e := &evs[j]; e.Kind == events.Resolve && unlessChainCounter(chName(g, e.Obj)) {
				st.resolves++
			}
		}
		evAt = len(evs)
		var top *state.Object
		if n := len(g.Stack); n > 0 {
			top = g.Obj(g.Stack[n-1])
		}
		if c.D.Kind != DPriority {
			if top != nil && top.Face() != nil && unlessChainCounter(top.Face().Name) {
				if top.Controller == state.PlayerID(c.D.Player) {
					st.ownAsks++
				} else {
					st.foreignAsks++
				}
			}
			return rnd.Pick(c)
		}
		buf = buf[:0]
		oppSpell := top != nil && top.Controller != state.PlayerID(c.D.Player)
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			if oppSpell {
				o, ok := c.G.GorgeOption(a)
				if !ok || !unlessChainCounter(chName(g, o.Obj)) {
					continue
				}
			}
			buf = append(buf, a)
		}
		if len(buf) > 0 && (oppSpell || prng.IntN(4) != 0) {
			return buf[prng.IntN(len(buf))]
		}
		return rnd.Pick(c)
	})
}

func TestUnlessChainLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unlesschain")
	if err != nil {
		t.Fatal(err)
	}
	var st unlessChainStats
	games, divs, stalls := 0, 0, 0
	const seeds = 16 // per seating per matchup: 4 x 16 = 64 games
	for mi, md0 := range unlessChainDecks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
			}
			var decks [2][]*cards.Card
			for s, d := range md {
				for _, e := range d {
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for i := 0; i < e.Count; i++ {
						decks[s] = append(decks[s], c)
					}
				}
				if len(decks[s]) != 40 {
					t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
				}
			}
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= seeds; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"deck-a", "deck-b"}, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				g, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				res := LockstepLed(m, seed, g, k, unlessChainPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls++
					t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					divs++
					t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if st.resolves < 3 || st.foreignAsks < 3 {
		t.Errorf("counter resolutions %d, unless asks to the countered spell's controller %d (want >= 3 each)", st.resolves, st.foreignAsks)
	}
}
