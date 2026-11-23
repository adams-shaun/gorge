package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// seanceScen builds a scenario over the named cards of the tail-24 pool.
func seanceScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	var deck []cardset.Entry
	set := cardsettest.Synth(t, "tail-24")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			return nil, nil
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Island"], Config{NoElide: true}), id
}

// TestSeanceBoardPaysAnXSpellAtXZero (CR 106.6, 601.2g; gorge's planner census):
// Seance Board's two soul counters make UU for a sorcery, so Island, Island and
// the Board pay Mass Manipulation's {X}{X}{U}{U}{U}{U} at X = 0. The X-bound
// target sweep (xBoundPayable) counts the Board's evaluated amount as a
// census unit as the planner does, and offers the cast; with no soul counter
// the Board makes nothing and the cast is withheld.
func TestSeanceBoardPaysAnXSpellAtXZero(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   []string
		souls int
		want  bool
	}{
		{"two Islands and the Board", []string{"Island", "Island", "Séance Board"}, 2, true},
		{"Island, Crag and the Board", []string{"Island", "Vivid Crag", "Séance Board"}, 2, true},
		{"three Islands and the Board", []string{"Island", "Island", "Island", "Séance Board"}, 2, true},
		{"the Board holds no soul counter", []string{"Island", "Island", "Séance Board"}, 0, false},
		{"one soul counter is one mana", []string{"Island", "Island", "Séance Board"}, 1, false},
	} {
		names := append([]string{"Island", "Mass Manipulation"}, tc.src...)
		sc, id := seanceScen(t, names...)
		if sc == nil {
			t.Skip("Mass Manipulation or the Board is not supported in the tail-24 table")
		}
		for _, n := range tc.src {
			p := &sc.s.perms[sc.slot(sc.perm(id[n], 0))]
			p.flags &^= pfTapped // (the scenario's enter applied Vivid Crag's enters-tapped replacement)
			switch n {
			case "Vivid Crag":
				p.setCtrN(sc.s, int(kop.CtrCharge), 2)
			case "Séance Board":
				p.setCtrN(sc.s, int(kop.CtrSoul), tc.souls)
			}
		}
		c := sc.hand(id["Mass Manipulation"], 0)
		d := sc.s.Step()
		if got := castOffered(&d, c); got != tc.want {
			t.Errorf("%s: Mass Manipulation offered %v, want %v", tc.name, got, tc.want)
		}
	}
}
