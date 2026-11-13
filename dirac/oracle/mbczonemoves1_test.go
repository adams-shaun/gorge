package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The MBC zone-moves-1 family (kgen census, spelling by spelling). Each spelling
// that lowers has a fixture-pool lockstep game here that plays one of its
// cards and fails without the lowering.
//
// Elda, Conjurer of Spectacle: "The first nonlegendary creature spell you cast
// each turn has demonstrate" is an AffectedZone$ Stack Demonstrate grant behind
// a CheckSVar$ X / SVarCompare$ EQ0 gate (X = Count$ThisTurnCast_Creature.YouCtrl+nonLegendary).
// gorge reads the gate against the casts PRIOR to the in-flight one
// (rules hasKeywordPriorCasts), so the first such spell poses the demonstrate
// election and the second, or a legendary one, does not.
var mbcZoneMoves1Elda = [2][]cardset.Entry{
	{
		{Name: "Elda, Conjurer of Spectacle", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6}, {Name: "Giant Spider", Count: 2},
		{Name: "Plains", Count: 9}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6}, {Name: "Giant Spider", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 12},
	},
}

// mbcZoneMoves1Run plays the Elda matchups (Elda in seat 0's deck, then in
// seat 1's) over seeds 1..seeds with a policy that answers every yes/no ask
// yes and casts whenever it can (kdemoPolicy). mutate, when set, edits each
// kernel table first. It returns the divergent games and the demonstrate
// copies the kernel minted (kdemoTally).
func mbcZoneMoves1Run(t *testing.T, seeds uint64, mutate func(*kop.Table)) (div, minted int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbczonemoves1")
	if err != nil {
		t.Fatal(err)
	}
	matchups := [][2][]cardset.Entry{mbcZoneMoves1Elda, {mbcZoneMoves1Elda[1], mbcZoneMoves1Elda[0]}}
	for mi, md := range matchups {
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
		if mutate != nil {
			mutate(tbl)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"mbc-a", "mbc-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tal := &kdemoTally{live: map[int]uint8{}}
			res := Lockstep(m, seed, k, kdemoPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: tal.check})
			CloseSide(k)
			if d := res.Div; d != nil {
				div++
				t.Logf("matchup %d seed %d: diverged at decision %d: %s %s %v %s", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail)
			}
			minted += tal.minted[0] + tal.minted[1]
		}
	}
	return div, minted
}

// TestMBCZoneMoves1Lowers: Elda's gated Demonstrate grant lowers and plays
// decision for decision, and the grant is really read (copies are minted).
func TestMBCZoneMoves1Lowers(t *testing.T) {
	t.Run("Elda", func(t *testing.T) {
		t.Parallel()
		div, minted := mbcZoneMoves1Run(t, 24, nil)
		if div != 0 {
			t.Fatalf("%d divergent games", div)
		}
		if minted < 8 {
			t.Fatalf("%d demonstrate copies minted over the games, want at least 8", minted)
		}
	})
}

// TestMBCZoneMoves1Mutations proves the lockstep reads both halves of the
// lowering: with the grants ignored the kernel never poses gorge's demonstrate
// election, and with the CheckSVar$ gate dropped it poses one for every
// nonlegendary creature spell, not the first.
func TestMBCZoneMoves1Mutations(t *testing.T) {
	dirac.DebugStackKwOff = true
	div, _ := mbcZoneMoves1Run(t, 24, nil)
	dirac.DebugStackKwOff = false
	if div == 0 {
		t.Fatal("ignoring the stack grants left the lockstep clean")
	}
	gated := 0
	div, _ = mbcZoneMoves1Run(t, 24, func(tb *kop.Table) {
		for i := range tb.Cards {
			for j := range tb.Cards[i].Abilities {
				if a := &tb.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatStackKw && a.Static.Cond2 != 0 {
					a.Static.Cond2 = 0
					gated++
				}
			}
		}
	})
	if gated == 0 {
		t.Fatal("no gated stack grant in the table to drop")
	}
	if div == 0 {
		t.Fatal("dropping the grant's gate left the lockstep clean")
	}
}
