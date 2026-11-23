package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The graveyard- and library-origin cast provenance lockstep (T13c): gorge
// ObjectWasCastFromGraveyard and castOriginTokens' wasCastFromYourLibrary on
// a spell on the stack, the two origins the kernel's PredWasCastSpell
// predates this work without. Ash Zealot triggers on a spell cast from a
// graveyard (a flashback cast of Otterball Antics or Antiquities on the Loose
// qualifies, a normal cast does not), Laquatus's Disdain may counter a spell
// cast from a graveyard, Melek, Izzet Paragon copies an instant or sorcery
// cast from your library and Wickerwing Effigy animates a creature spell cast
// from your library, both reading the free casts Cosmic Cube offers from the
// top of its controller's library after attacking.

var kgraveprovDecks = [][2][]cardset.Entry{
	{ // M1: the graveyard origin
		{ // A: R/U/W
			{Name: "Ash Zealot", Count: 3}, {Name: "Laquatus's Disdain", Count: 3},
			{Name: "Otterball Antics", Count: 4}, {Name: "Antiquities on the Loose", Count: 3},
			{Name: "Savannah Lions", Count: 2}, {Name: "Healer's Hawk", Count: 2},
			{Name: "Coral Merfolk", Count: 2}, {Name: "Opt", Count: 2},
			{Name: "Mountain", Count: 8}, {Name: "Island", Count: 5}, {Name: "Plains", Count: 6},
		},
		{ // B: U/R
			{Name: "Ash Zealot", Count: 2}, {Name: "Laquatus's Disdain", Count: 4},
			{Name: "Otterball Antics", Count: 4}, {Name: "Shock", Count: 3},
			{Name: "Opt", Count: 2}, {Name: "Coral Merfolk", Count: 3},
			{Name: "Hill Giant", Count: 2}, {Name: "Memnite", Count: 2},
			{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
		},
	},
	{ // M2: the library origin
		{ // A: U/R, Melek over Cosmic Cube's free casts
			{Name: "Melek, Izzet Paragon", Count: 2}, {Name: "Cosmic Cube", Count: 2},
			{Name: "Wickerwing Effigy", Count: 2}, {Name: "Opt", Count: 3},
			{Name: "Shock", Count: 3}, {Name: "Divination", Count: 2},
			{Name: "Coral Merfolk", Count: 3}, {Name: "Ornithopter", Count: 2},
			{Name: "Memnite", Count: 2}, {Name: "Hill Giant", Count: 2},
			{Name: "Island", Count: 9}, {Name: "Mountain", Count: 8},
		},
		{ // B: U/R mirrors without Melek: the words read false over its casts
			{Name: "Cosmic Cube", Count: 2}, {Name: "Wickerwing Effigy", Count: 2},
			{Name: "Opt", Count: 3}, {Name: "Shock", Count: 3},
			{Name: "Divination", Count: 2}, {Name: "Ornithopter", Count: 2},
			{Name: "Memnite", Count: 2}, {Name: "Coral Merfolk", Count: 3},
			{Name: "Hill Giant", Count: 3},
			{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
		},
	},
}

// kgraveprovRun plays seeds 1..seeds of both matchups (the surface alternates
// with the seed) and returns whether a game diverged, the kernel capacity
// stall count and the predicate's coverage counters.
func kgraveprovRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st dirac.WasCastSpellStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("wascastgraveprov")
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugWasCastSpellStats = &st
	defer func() { dirac.DebugWasCastSpellStats = nil }()
	for mi, md := range kgraveprovDecks {
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
			m, err := NewMatchup(reg, [2]string{"kgraveprov-a", "kgraveprov-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 30})
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
			}
			if d := res.Div; d != nil {
				diverged = true
				if report {
					lo := d.At - 12
					if lo < 0 {
						lo = 0
					}
					t.Logf("m%d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("m%d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				return
			}
		}
	}
	return
}

// TestWascastGraveprovLockstep: no divergence over 64 seeds of each matchup,
// both surfaces, and both new origins read.
func TestWascastGraveprovLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, stalls, st := kgraveprovRun(t, 64, true)
	t.Logf("128 games, graveprov %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if div {
		t.Fatal("diverged")
	}
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.FromGrave, 20, "reads of a spell cast from a graveyard"},
		{st.FromLibrary, 20, "reads of a spell cast from a library"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}
