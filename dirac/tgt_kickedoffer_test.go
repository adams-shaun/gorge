package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// offersKickedCast reports whether the decision offers o's kicked cast (the
// AkCast action whose mode bit 0 is set) and its plain cast.
func offersKickedCast(s *State, o Obj) (kicked, plain bool) {
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkCast || Obj(a.Source()) != o {
			continue
		}
		if a.Mode()&1 != 0 { // seat-ok: a cast-mode bit
			kicked = true
		} else {
			plain = true
		}
	}
	return kicked, plain
}

// TestKickedUniqueChainCastOffer: Urborg Repossession's kicked cast also
// returns "another target permanent card from your graveyard" (TargetUnique$,
// owed only when kicked), so one permanent card in the graveyard leaves it no
// distinct target: the plain cast is offered, the kicked one is not (gorge
// kickedChainTargetsAvailable; offering it could only end in the CR 733.1
// reversal). A second permanent card offers it.
func TestKickedUniqueChainCastOffer(t *testing.T) {
	deck := []cardset.Entry{{Name: "Urborg Repossession", Count: 1}, {Name: "Novice Occultist", Count: 2},
		{Name: "Swamp", Count: 20}, {Name: "Forest", Count: 20}}
	tbl, da, _, err := cardsettest.Synth(t, "dr-mbc-baba").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, tc := range []struct {
		permanents int
		want       bool
	}{{1, false}, {2, true}} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		for i := 0; i < 3; i++ {
			sc.perm(id["Swamp"], 0)
			sc.perm(id["Forest"], 0)
		}
		sc.grave(id["Novice Occultist"], 0, tc.permanents)
		rep := sc.hand(id["Urborg Repossession"], 0)
		kicked, plain := offersKickedCast(sc.s, rep)
		if !plain {
			t.Fatalf("%d permanent cards: the plain cast must be offered (precondition)", tc.permanents)
		}
		if kicked != tc.want {
			t.Fatalf("%d permanent cards in the graveyard: kicked cast offered = %v, want %v", tc.permanents, kicked, tc.want)
		}
	}
}
