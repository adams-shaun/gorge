package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Effect-delivered top-of-library permission in lockstep: The Belligerent
// (crewed and attacking, then lands and spells from the top of the library)
// and Gwenom, Remorseless (cards from the top, a spell paid for with life
// equal to its mana value). A policy that prefers the top card plays it
// whenever the permission offers it, so the offered top card, its price and
// the life paid are checked against gorge's at every decision.

var maylookTopDecks = []struct {
	name  string
	decks [2][]cardset.Entry
}{{
	"belligerent", [2][]cardset.Entry{
		{
			{Name: "The Belligerent", Count: 4}, {Name: "Hill Giant", Count: 5}, {Name: "Gray Ogre", Count: 4},
			{Name: "Burst Lightning", Count: 3}, {Name: "Think Twice", Count: 3}, {Name: "Island", Count: 11}, {Name: "Mountain", Count: 10},
		},
		{
			{Name: "Hill Giant", Count: 8}, {Name: "Gray Ogre", Count: 8}, {Name: "Burst Lightning", Count: 6}, {Name: "Mountain", Count: 18},
		},
	},
}, {
	"gwenom", [2][]cardset.Entry{
		{
			{Name: "Gwenom, Remorseless", Count: 5}, {Name: "Walking Corpse", Count: 8}, {Name: "Doom Blade", Count: 4}, {Name: "Swamp", Count: 23},
		},
		{
			{Name: "Walking Corpse", Count: 8}, {Name: "Doom Blade", Count: 4}, {Name: "Swamp", Count: 28},
		},
	},
}}

// runMayLookTop plays each matchup over seeds 1..seeds and returns the
// per-matchup counts of top-of-library plays.
func runMayLookTop(t *testing.T, seeds int) map[string]klibtopStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("maylooktop")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]klibtopStats{}
	for _, md := range maylookTopDecks {
		var decks [2][]*cards.Card
		for s, d := range md.decks {
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
				t.Fatalf("%s deck %d: %d cards", md.name, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md.decks[0], md.decks[1])
		if err != nil {
			t.Fatal(err)
		}
		var st klibtopStats
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{md.name + "-a", md.name + "-b"}, decks, 1)
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
				t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", md.name, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		out[md.name] = st
	}
	return out
}

func TestMayLookTopLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for name, st := range runMayLookTop(t, 128) {
		t.Logf("%s: %d games, %d divergences, %d casts and %d land plays from the top of a library", name, st.games, st.divs, st.libCasts, st.libLands)
		if st.libCasts < 10 || st.libLands < 5 {
			t.Errorf("%s: thin coverage: %d casts, %d land plays (want >= 10, 5)", name, st.libCasts, st.libLands)
		}
	}
}
