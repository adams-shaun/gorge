package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Nog, Ambitious Cadet lockstep: the Officer step permanently grants
// "You may play lands from your graveyard" and "You may play an additional
// land on each of your turns" (an Animate staticAbilities$ list). Seat 1's
// Stone Rain and Mind Rot put seat 0's lands in its graveyard; the kernel
// must offer the same land plays (from the graveyard, and the second land of
// a turn) as gorge at every decision, and offer neither before the Officer
// step.

var knogDecks = [2][]cardset.Entry{{
	{Name: "Nog, Ambitious Cadet", Count: 8}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Forest", Count: 15}, {Name: "Swamp", Count: 15},
}, {
	{Name: "Stone Rain", Count: 8}, {Name: "Mind Rot", Count: 6}, {Name: "Hill Giant", Count: 4},
	{Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 11},
}}

// knogStats counts what the games reached: decisions with an Officer Nog on
// the battlefield, land plays from a graveyard, and second land plays of a turn.
type knogStats struct {
	officer, grave, extra int
}

// knogPolicy plays a land first (from the graveyard when it can), then
// casts or activates, else random; it counts the Officer-gated plays.
func knogPolicy(seed uint64, st *knogStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	landTurn, landN := -1, 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			for i := range g.Objs {
				o := &g.Objs[i]
				if o.Zone == state.ZBattlefield && o.Face() != nil && o.Face().Name == "Nog, Ambitious Cadet" {
					for _, ty := range c.G.E.Chars(o.ID).Types {
						if ty == "Officer" {
							st.officer++
							break
						}
					}
					break
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
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
		if a.Kind() == ActPlayLand && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil {
					if ob.Zone == state.ZGraveyard {
						st.grave++
					}
					if t := c.G.Turn(); t != landTurn {
						landTurn, landN = t, 0
					}
					if landN++; landN > 1 {
						st.extra++
					}
				}
			}
		}
		return a
	})
}

// runNog plays the matchup for seeds 1..seeds and returns the games,
// divergences, stalls and what they reached. mutate withholds the granted
// statics from the kernel.
func runNog(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st knogStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("nog")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugNogNoGrant
	dirac.DebugNogNoGrant = mutate
	defer func() { dirac.DebugNogNoGrant = old }()
	var decks [2][]*cards.Card
	for s, d := range knogDecks {
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
	tbl, _, _, err := set.Matchup(knogDecks[0], knogDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"knog-a", "knog-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, knogPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return
}

func TestNogAmbitiousCadetLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runNog(t, 128, false)
	t.Logf("%d games, %d divergences, %d stalls; %d decisions with an Officer Nog, %d land plays from a graveyard, %d second land plays of a turn", games, divs, stalls, st.officer, st.grave, st.extra)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.officer == 0 || st.grave < 10 || st.extra < 10 {
		t.Errorf("the games barely reached the grant: %d Officer decisions, %d graveyard land plays, %d second land plays (want at least 10 of each play)", st.officer, st.grave, st.extra)
	}
}

// TestNogAmbitiousCadetMutation withholds the granted graveyard land
// permission and the extra land play from the kernel: the games must diverge.
// A green mutation (no divergence) means the lockstep never reaches either.
func TestNogAmbitiousCadetMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runNog(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation withheld every granted static in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
