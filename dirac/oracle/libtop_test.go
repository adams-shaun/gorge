package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The top-of-library play lockstep: Ka-Zar (lands),
// Assemble the Players (power <= 2 nonland creatures, once a turn), Hakoda
// (Allies), Johann (instants and sorceries, once a turn) and Madame Web
// (Spiders, noncreature spells) let their controller play or cast the top
// card of their own library. Every deck card is one some permission
// matches or skips, so the offered top card is checked against gorge's at
// every decision. gorge caps MayPlayLimit$ once per turn per static
// (engine/rules/mayplay.go mayPlayKeyLimitReached), so Assemble
// the Players and Johann play one top card a turn each; a turn still sees
// two plays through two permissions (Ka-Zar's land and Assemble's creature,
// or two copies of Assemble). The mutation offers the second card instead.

var klibtopDecks = [][2][]cardset.Entry{{
	{
		{Name: "Ka-Zar of the Savage Land", Count: 3}, {Name: "Assemble the Players", Count: 3},
		{Name: "Hakoda, Selfless Commander", Count: 3}, {Name: "Expedition Envoy", Count: 3},
		{Name: "Kyoshi Warrior Guard", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Walking Corpse", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Gray Ogre", Count: 2}, {Name: "Giant Growth", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Plains", Count: 7},
	},
	{
		{Name: "Johann, Apprentice Sorcerer", Count: 4}, {Name: "Madame Web, Clairvoyant", Count: 3},
		{Name: "Burst Lightning", Count: 3}, {Name: "Sure Strike", Count: 2}, {Name: "Think Twice", Count: 3},
		{Name: "Boltwave", Count: 2}, {Name: "Divination", Count: 2}, {Name: "Canopy Spider", Count: 2},
		{Name: "Giant Spider", Count: 2}, {Name: "Island", Count: 7}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 4},
	},
}}

// klibtopLimited names the once-a-turn carriers.
var klibtopLimited = map[string]bool{"Assemble the Players": true, "Johann, Apprentice Sorcerer": true}

type klibtopStats struct {
	games, divs, libCasts, libLands, spentTurns, twoPlays int
	twoCasts                                              int // decisions offering two casts of one top card (maylooktop_lockstep_test.go)
}

// klibtopPolicy plays whenever it can, preferring the card on top of a
// library, and counts those plays (and the turns a once-a-turn permission
// was already spent when the player decided again) on the gorge side.
func klibtopPolicy(seed uint64, st *klibtopStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	type turnKey struct {
		turn int
		p    uint8
	}
	spent := map[turnKey]bool{}
	counted := map[turnKey]bool{}
	plays := map[turnKey]int{}
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		var key turnKey
		if g != nil {
			key = turnKey{g.Turn(), c.D.Player}
			if spent[key] && !counted[key] {
				counted[key] = true
				st.spentTurns++
			}
		}
		buf = buf[:0]
		lib := false
		for _, a := range c.D.Options {
			k := a.Kind()
			if k != ActCast && k != ActPlayLand {
				continue
			}
			fromLib := false
			if g != nil {
				if o, ok := g.GorgeOption(a); ok {
					if ob := g.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZLibrary {
						fromLib = true
					}
				}
			}
			if fromLib && !lib {
				buf, lib = buf[:0], true
			}
			if fromLib == lib {
				buf = append(buf, a)
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
		if k := a.Kind(); (k == ActCast || k == ActPlayLand) && g != nil {
			if o, ok := g.GorgeOption(a); ok {
				if ob := g.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZLibrary {
					if k == ActCast {
						st.libCasts++
					} else {
						st.libLands++
					}
					if plays[key]++; plays[key] == 2 {
						st.twoPlays++ // a second top card through another permission
					}
					for i := range g.E.G.Objs {
						p := &g.E.G.Objs[i]
						if p.Zone == state.ZBattlefield && p.Face() != nil && klibtopLimited[p.Face().Name] && uint8(p.Controller) == c.D.Player {
							spent[key] = true
						}
					}
				}
			}
		}
		return a
	})
}

// runLibtop plays the constructed matchup over seeds 1..seeds. mutate
// sets DebugLibTopAnyCard (the mutation must diverge).
func runLibtop(t *testing.T, seeds int, mutate bool) klibtopStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("libtop")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugLibTopAnyCard
	dirac.DebugLibTopAnyCard = mutate
	defer func() { dirac.DebugLibTopAnyCard = old }()
	var st klibtopStats
	for mi, md := range klibtopDecks {
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
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"klibtop-a", "klibtop-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, klibtopPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			st.games++
			if d := res.Div; d != nil {
				st.divs++
				if !mutate {
					t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return st
}

func TestLibtopLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runLibtop(t, 256, false)
	t.Logf("%d games, %d divergences, %d casts and %d land plays from the top of a library, %d turns a once-a-turn permission's carrier had already played one, %d turns with two plays",
		st.games, st.divs, st.libCasts, st.libLands, st.spentTurns, st.twoPlays)
	if st.divs != 0 {
		t.Fatalf("%d divergences", st.divs)
	}
	if st.libCasts < 30 || st.libLands < 15 || st.spentTurns < 10 || st.twoPlays < 10 {
		t.Errorf("thin coverage: %d casts, %d land plays, %d spent turns, %d double-play turns (want >= 30, 15, 10, 10)", st.libCasts, st.libLands, st.spentTurns, st.twoPlays)
	}
}

// TestLibtopMutation offers the second card from the top: gorge offers
// only the top one, so the games must diverge.
func TestLibtopMutation(t *testing.T) {
	st := runLibtop(t, 32, true)
	if st.divs == 0 {
		t.Fatalf("mutation offered the second library card in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
