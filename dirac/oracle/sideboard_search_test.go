package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The searches that reach outside the game (CR 400.11): North Wind Avatar's
// "put a card you own from outside the game into your hand" and Turtles
// Forever's "search your library and/or outside the game for exactly four
// legendary creature cards with different names ... an opponent chooses two
// ... shuffle the rest into your library". Each seat carries a real sideboard
// in both engines, and the kernel must pose the same asks over the same
// library-then-sideboard pools, and leave the same cards in the same zones
// and the same library order. The std-tmt synthetic pool holds both cards;
// that pool's kfuzz decks carry no sideboards (the kernel's synth Config has
// none), so the games here are the sideboard coverage.

// sideLegends returns n legendary creature cards of set, in name order from
// skip: the cards the library and sideboard searches fetch.
func sideLegends(t *testing.T, reg *cards.Registry, set cardset.Set, skip, n int) []string {
	t.Helper()
	var out []string
	plays := set.Supported()
	for i, name := range set.Names()[:set.NumCards()] {
		if !plays[i] || strings.Contains(name, ",") && strings.Contains(name, "//") {
			continue
		}
		c, ok := reg.Lookup(name)
		if !ok || len(c.Faces) != 1 {
			continue
		}
		ty := strings.Join(c.Faces[0].Types, " ")
		if !strings.Contains(ty, "Legendary") || !strings.Contains(ty, "Creature") || len(c.Faces[0].Abilities)+len(c.Faces[0].Triggers) > 4 {
			continue
		}
		if skip > 0 {
			skip--
			continue
		}
		out = append(out, name)
		if len(out) == n {
			return out
		}
	}
	t.Fatalf("the set has only %d legendary creature cards after %d skipped, want %d", len(out), skip, n)
	return nil
}

func sideEntries(names []string, count int) []cardset.Entry {
	var es []cardset.Entry
	for _, n := range names {
		es = append(es, cardset.Entry{Name: n, Count: count})
	}
	return es
}

type sideMatchup struct {
	deck [2][]cardset.Entry
	side [2][]cardset.Entry
}

func sideMatchups(t *testing.T, reg *cards.Registry, set cardset.Set) []sideMatchup {
	lib := sideLegends(t, reg, set, 0, 11)
	out := sideLegends(t, reg, set, 11, 7)
	seat0 := append([]cardset.Entry{{Name: "North Wind Avatar", Count: 4}, {Name: "Turtles Forever", Count: 4},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Plains", Count: 7}}, sideEntries(lib, 1)...)
	filler := append([]cardset.Entry{{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12}}, sideEntries(lib[4:], 1)...)
	filler = append(filler, sideEntries(lib[:1], 9)...)
	return []sideMatchup{{
		// seat 0 holds both cards and a sideboard of legends and lands; the opponent's has none
		deck: [2][]cardset.Entry{seat0, filler},
		side: [2][]cardset.Entry{append(sideEntries(out[:6], 1), cardset.Entry{Name: "Island", Count: 2}), nil},
	}, {
		// both seats search outside the game; the opponent chooses from a pool that holds legends twice over
		deck: [2][]cardset.Entry{seat0, append(append([]cardset.Entry{{Name: "North Wind Avatar", Count: 3}, {Name: "Turtles Forever", Count: 3},
			{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8}}, sideEntries(lib[4:], 1)...), sideEntries(lib[:3], 1)...)},
		side: [2][]cardset.Entry{sideEntries(out[:3], 1), append(sideEntries(out[3:7], 1), cardset.Entry{Name: "Plains", Count: 1})},
	}}
}

func expandSide(t *testing.T, reg *cards.Registry, es []cardset.Entry) []*cards.Card {
	t.Helper()
	var out []*cards.Card
	for _, e := range es {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			out = append(out, c)
		}
	}
	return out
}

// sideReach counts what the games did with the sideboards, read from gorge's
// event log: cards moved from a sideboard to a hand (both cards' picks) and
// into a library (Turtles Forever's put-back), and Turtles Forever casts.
type sideReach struct{ toHand, toLibrary, tfCasts, nwaCasts int }

// runSideSearch plays the matchups for seeds 1..seeds; it returns the games,
// the divergences, the stalls and the games in which a card left a sideboard.
func runSideSearch(t *testing.T, seeds int, mutate bool) (games, divs, stalls, fetched int, reach sideReach) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tmt")
	old := dirac.DebugSideSearchLibraryOnly
	dirac.DebugSideSearchLibraryOnly = mutate
	defer func() { dirac.DebugSideSearchLibraryOnly = old }()
	for mi, md := range sideMatchups(t, reg, set) {
		var decks, sides [2][]*cards.Card
		for s := range md.deck {
			decks[s] = expandSide(t, reg, md.deck[s])
			sides[s] = expandSide(t, reg, md.side[s])
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(append(append([]cardset.Entry(nil), md.deck[0]...), md.side[0]...), append(append([]cardset.Entry(nil), md.deck[1]...), md.side[1]...))
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchupSide(reg, [2]string{fmt.Sprintf("side-search-%d-a", mi), fmt.Sprintf("side-search-%d-b", mi)}, decks, sides, 1)
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
			casts := map[string]int{}
			pol := poisonhighPolicy(seed, casts, map[string]int{}) // casts whenever it can
			res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
			CloseSide(k)
			games++
			reach.tfCasts += casts["Turtles Forever"]
			reach.nwaCasts += casts["North Wind Avatar"]
			for _, e := range g.E.L.Events {
				if e.Kind == events.MoveZone && e.From == state.ZSideboard {
					switch e.To {
					case state.ZHand:
						reach.toHand++
					case state.ZLibrary:
						reach.toLibrary++
					}
				}
			}
			for p := range sides {
				if len(g.E.G.Zone(state.ZSideboard, state.PlayerID(p))) < len(sides[p]) {
					fetched++
					break
				}
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return
}

func TestSideboardSearchLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, fetched, reach := runSideSearch(t, 96, false)
	t.Logf("%d games, %d divergences, %d stalls, %d games with a card taken from a sideboard; %+v", games, divs, stalls, fetched, reach)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if fetched < 20 {
		t.Errorf("only %d games took a card from a sideboard (want at least 20)", fetched)
	}
	if reach.toHand < 20 || reach.toLibrary < 5 || reach.tfCasts < 20 || reach.nwaCasts < 20 {
		t.Errorf("reach %+v: want sideboard cards into a hand and a library, and both cards cast, at least 20/5/20/20 times", reach)
	}
}

// TestSideboardSearchMutation drops the kernel's sideboard from every pool:
// the games must diverge. A green mutation means the lockstep never searches
// outside the game.
func TestSideboardSearchMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _, _ := runSideSearch(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation dropped the sideboard from every search in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
