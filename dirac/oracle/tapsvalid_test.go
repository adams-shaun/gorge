package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The "whenever you tap an untapped creature an opponent controls" lockstep
// the four WOE Taps ValidPlayer$ You carriers and the
// WOE tap effects (spells, activated taps, enter triggers, an aura) against
// a creature deck that attacks every turn. Its attacks tap its own
// creatures (tapper = B), which must not fire A's lines; A's tap effects
// tap B's creatures as A (gorge TapActor), which must.

var ktapsvalidDecks = [2][]cardset.Entry{
	{
		{Name: "Hylda of the Icy Crown", Count: 3}, {Name: "Icewrought Sentry", Count: 4},
		{Name: "Sharae of Numbing Depths", Count: 3}, {Name: "Solitary Sanctuary", Count: 3},
		{Name: "Freeze in Place", Count: 2}, {Name: "Frostbridge Guard", Count: 3}, {Name: "Succumb to the Cold", Count: 2},
		{Name: "Plunge into Winter", Count: 1}, {Name: "Hylda's Crown of Winter", Count: 1}, {Name: "Bitter Chill", Count: 1},
		{Name: "Snaremaster Sprite", Count: 1}, {Name: "Savannah Lions", Count: 1}, {Name: "Wind Drake", Count: 1},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 7},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 5}, {Name: "Gray Ogre", Count: 6},
		{Name: "Giant Spider", Count: 6}, {Name: "Forest", Count: 9}, {Name: "Mountain", Count: 8},
	},
}

// ktapsvalidTrig names each carrier's Taps trigger body (its API), so a
// stack object of the carrier with that API is its Taps trigger.
var ktapsvalidTrig = map[string]string{
	"Hylda of the Icy Crown":   "ImmediateTrigger",
	"Icewrought Sentry":        "Pump",
	"Sharae of Numbing Depths": "Draw",
	"Solitary Sanctuary":       "PutCounter",
}

type ktapsvalidStats struct {
	trigs    int // carrier Taps triggers seen on the stack
	oppAtks  int // seat 1 attack declarations while a seat 0 carrier was on the battlefield
	byCard   map[string]int
	seenTrig map[state.ObjID]bool
}

// ktapsvalidPolicy attacks whenever it can, else casts or activates, else
// random; it counts the carriers' Taps triggers and B's attacks into them.
func ktapsvalidPolicy(seed uint64, st *ktapsvalidStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			for _, sid := range g.Stack {
				ob := g.Obj(sid)
				if ob == nil || ob.Ability == nil || st.seenTrig[sid] {
					continue
				}
				if src := g.Obj(ob.Source); src != nil && src.Face() != nil {
					if api, ok := ktapsvalidTrig[src.Face().Name]; ok && ob.Ability.API == api {
						st.seenTrig[sid] = true
						st.trigs++
						st.byCard[src.Face().Name]++
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActAttack {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate {
					buf = append(buf, a)
				}
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActAttack && c.D.Player == 1 && c.G != nil {
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, 0) {
				if ob := c.G.E.G.Obj(id); ob != nil && ob.Face() != nil {
					if _, ok := ktapsvalidTrig[ob.Face().Name]; ok {
						st.oppAtks++
						break
					}
				}
			}
		}
		return a
	})
}

// runTapsvalid plays seeds 1..seeds and returns the games, divergences,
// stalls and stats. mutate ignores the tapper (DebugTapsValidAnyTapper).
func runTapsvalid(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st *ktapsvalidStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tapsvalid")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugTapsValidAnyTapper
	dirac.DebugTapsValidAnyTapper = mutate
	defer func() { dirac.DebugTapsValidAnyTapper = old }()
	st = &ktapsvalidStats{byCard: map[string]int{}}
	var decks [2][]*cards.Card
	for s, d := range ktapsvalidDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(ktapsvalidDecks[0], ktapsvalidDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		st.seenTrig = map[state.ObjID]bool{}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktapsvalid-a", "ktapsvalid-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, ktapsvalidPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, st
}

func TestTapsvalidLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, st := runTapsvalid(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d carrier Taps triggers %v, %d opponent attacks into a carrier", games, divs, stalls, st.trigs, st.byCard, st.oppAtks)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.trigs < 40 {
		t.Errorf("only %d carrier Taps triggers", st.trigs)
	}
	if st.oppAtks < 40 {
		t.Errorf("only %d opponent attacks while a carrier was on the battlefield", st.oppAtks)
	}
}

// TestTapsvalidMutation ignores the tapper: B's attack taps then fire A's
// "whenever you tap" lines, so the games must diverge.
func TestTapsvalidMutation(t *testing.T) {
	games, divs, _, _ := runTapsvalid(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation ignored the tapper in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
