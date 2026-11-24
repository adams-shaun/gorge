package oracle

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// scriptedStartSeeds is how many game seeds every fixture plays (T4: "clean
// over 8 seeds").
const scriptedStartSeeds = 8

// scriptedStartPool names the kernel pool a fixture's table is built from. A
// fixture not listed plays on the FDN table. The FDN pool holds no
// multi-face card (its table has no Faces), so the back-face fixture is the
// one that cannot, and plays on the pauper table, where Sagu Wildling is a
// modal DFC with a land back face.
var scriptedStartPool = map[string]string{
	"sagu-wildling.4.json": "pauper",
}

// TestScriptedStartLockstep plays every fixture under testdata/scenarios in
// lockstep, gorge leading and the real kernel following, over
// scriptedStartSeeds game seeds each. A scripted start reaches both engines
// as the same Matchup.Start (gorge through rules.Config.Start, the kernel
// through RealKernel's InitNStart), so the first compare, at decision 0,
// checks placement, tapped, counters, back face, attachment, library
// position, life and the active seat. A run must be clean to its end.
//
// Each game also checks what the compare cannot: the engine's starting
// player is the scenario's active seat (D7), and the placements are in
// gorge's state, so a fixture whose start did nothing fails rather than
// passing vacuously.
func TestScriptedStartLockstep(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "scenarios", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 6 {
		t.Fatalf("%d fixtures under testdata/scenarios: want at least 6", len(files))
	}
	slices.Sort(files)
	c, err := OpenCorpus(corpusDirOrSkip(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(strings.TrimSuffix(name, ".json"), func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			sc, err := ParseScenario(raw)
			if err != nil {
				t.Fatal(err)
			}
			m, err := c.ScenarioMatchup(sc, 0)
			if err != nil {
				t.Fatal(err)
			}
			pool := scriptedStartPool[name]
			if pool == "" {
				pool = "fdn"
			}
			set, tbl := scriptedStartTable(t, pool, m)
			cfg := Config{}
			rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 4}
			next := uint64(1)
			for i := 0; i < scriptedStartSeeds; i++ {
				seed := sc.GameSeed(next)
				next = seed + 1
				lead, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				if got := int(lead.E.G.StartingPlayer); got != sc.Active {
					t.Fatalf("seed %d: starting player %d, the scenario wants %d", seed, got, sc.Active)
				}
				scriptedStartPlaced(t, lead, m, seed)
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				res := LockstepLed(m, seed, lead, k, RandomPolicy(seed), rc)
				CloseSide(k)
				if res.Div != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v (%s)",
						seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
				}
				if res.Decisions == 0 {
					t.Fatalf("seed %d: no decision compared", seed)
				}
			}
			// The control: a kernel that ignores the start diverges at the
			// first compare, so the lockstep above does see the start.
			seed := sc.GameSeed(1)
			bare := *m
			bare.Start = nil
			k, err := SetSide(set, tbl, cfg)(&bare, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
			CloseSide(k)
			if res.Div == nil || res.Div.At != 0 {
				t.Fatalf("a kernel without the start did not diverge at decision 0: %+v", res.Div)
			}
		})
	}
}

// scriptedStartTable builds the pool's kernel table over m's two decks.
func scriptedStartTable(t *testing.T, pool string, m *Matchup) (cardset.Set, *kop.Table) {
	t.Helper()
	set, err := cardset.RawByName(pool)
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skip("no kernel corpus table: " + err.Error())
	}
	if err != nil {
		t.Fatal(err)
	}
	entries := func(d []*cards.Card) []cardset.Entry {
		var out []cardset.Entry
		for _, c := range d {
			n := cardName(c)
			i := slices.IndexFunc(out, func(e cardset.Entry) bool { return e.Name == n })
			if i < 0 {
				out = append(out, cardset.Entry{Name: n})
				i = len(out) - 1
			}
			out[i].Count++
		}
		return out
	}
	tbl, _, _, err := set.Matchup(entries(m.SeatDeck(0)), entries(m.SeatDeck(1)))
	if err != nil {
		t.Fatalf("pool %s cannot hold the fixture: %v", pool, err)
	}
	return set, tbl
}

// scriptedStartPlaced checks gorge's state at decision 0: every seat's zones
// hold the cards the start places (battlefield counts the attached Auras),
// the listed library cards are on top in the listed order, and the life total
// is the start's, so a start that did nothing fails here, not vacuously.
func scriptedStartPlaced(t *testing.T, g *GorgeSide, m *Matchup, seed uint64) {
	t.Helper()
	for p, sd := range m.Start.Seats {
		pid := state.PlayerID(p)
		zones := []struct {
			name string
			z    state.Zone
			want int
		}{
			{"battlefield", state.ZBattlefield, len(sd.Battlefield)},
			{"graveyard", state.ZGraveyard, len(sd.Graveyard)},
			{"exile", state.ZExile, len(sd.Exile)},
		}
		for _, z := range zones {
			if got := len(g.E.G.Zone(z.z, pid)); got != z.want {
				t.Fatalf("seed %d p%d: %d cards in %s, the start places %d", seed, p, got, z.name, z.want)
			}
		}
		if got := len(g.E.G.Zone(state.ZHand, pid)); got != len(sd.Hand) {
			t.Fatalf("seed %d p%d: %d cards in hand, the start places %d", seed, p, got, len(sd.Hand))
		}
		lib := g.E.G.Zone(state.ZLibrary, pid)
		deck := m.SeatDeck(p)
		for i, ix := range sd.Library {
			if i >= len(lib) || cardName(g.E.G.Obj(lib[i]).Card) != cardName(deck[ix]) {
				t.Fatalf("seed %d p%d: library position %d is not %s", seed, p, i, cardName(deck[ix]))
			}
		}
		if sd.Life != nil && int32(g.E.G.Players[p].Life) != *sd.Life {
			t.Fatalf("seed %d p%d: life %d, the start sets %d", seed, p, g.E.G.Players[p].Life, *sd.Life)
		}
	}
}
