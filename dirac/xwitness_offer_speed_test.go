package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXWitnessOfferSpeedReadsTheOfferedSpell: the offer path runs the
// witness's pre-cast trigger walk on a fork with no build live. bld() then
// holds stale bytes, so the walk must take the cast's speed from the spell
// being priced, not from whichever entry the scratch arm last held.
//
// Al Bhed Salvagers turns a Treasure's sacrifice into a life loss the pre-cast
// walk resolves; with the opponent at 1 life that ends the game, so a sorcery
// at the only source is not offered while an instant is.
func TestXWitnessOfferSpeedReadsTheOfferedSpell(t *testing.T) {
	set := cardsettest.Synth(t, "std-fin")
	deck := []cardset.Entry{
		{Name: "Choco-Comet", Count: 1},
		{Name: "Syncopate", Count: 1},
		{Name: "Al Bhed Salvagers", Count: 1},
		{Name: "Namazu Trader", Count: 1}, // its token list holds the Treasure
		{Name: "Mountain", Count: 1},
	}
	for _, e := range deck {
		if _, ok := set.Lookup(e.Name); !ok {
			t.Skipf("%s not in the std-fin table", e.Name)
		}
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	for _, tc := range []struct {
		name     string
		card     string
		stale    string // the other-speed entry left in the dead scratch arm
		mountain bool   // a second source would make a smaller X keep the Treasure
		offered  bool
	}{
		{"sorcery after an instant build", "Choco-Comet", "Syncopate", true, false},
		{"instant after a sorcery build", "Syncopate", "Choco-Comet", false, true},
	} {
		sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
		sc.perm(id["Al Bhed Salvagers"], 0)
		tr := sc.perm(id["c_a_treasure_sac"], 0)
		sc.s.objs[tr].of |= ofToken
		if tc.mountain {
			sc.perm(id["Mountain"], 0)
		}
		sc.s.pl[1].life = 1
		c, stale := id[tc.card], id[tc.stale]
		o := sc.hand(c, 0)
		if tc.card == "Syncopate" {
			// Syncopate targets a spell: one on the stack gives the walk a legal target.
			dummy := sc.obj(id["Al Bhed Salvagers"], 1)
			sc.s.objs[dummy].zone = kop.ZoneStack
			sc.s.stack[0] = stackEntry{kind: seSpell, obj: dummy, card: id["Al Bhed Salvagers"], ctrl: 1}
			sc.s.sp = 1
		}
		if sc.s.buildLive() {
			t.Fatalf("%s: precondition: a build is live at the offer walk", tc.name)
		}
		if k := sc.k; k.cards[c].instant == k.cards[stale].instant {
			t.Fatalf("%s: precondition: %s and %s are the same speed", tc.name, tc.card, tc.stale)
		}
		*sc.s.bld() = build{e: stackEntry{kind: seSpell, card: stale, ctrl: 0}}
		if got := sc.s.xWitnessOffered(sc.k, 0, c, o); got != tc.offered {
			t.Errorf("%s: offered %v, want %v", tc.name, got, tc.offered)
		}
	}
}
