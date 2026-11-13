package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// A granted prowess fires once per granting static (CR 702.108b, 113.2c), after
// a printed prowess's own face trigger: under Bria and Sokka an Ally token is
// granted prowess twice, Sokka (printed prowess) once, Bria (printed, and
// "other creatures") not at all.
func TestProwessGrantsPerStatic(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Bria, Riptide Rogue", Count: 4}, {Name: "Sokka, Tenacious Tactician", Count: 4},
		{Name: "Heartfire Immolator", Count: 4}, {Name: "Shock", Count: 8},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 8},
	}
	set := cardsettest.Set(t, "prowess")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var ally CardID
	found := false
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "w_1_1_ally" {
			ally, found = CardID(i), true
		}
	}
	if !found {
		t.Fatal("no w_1_1_ally token in the table")
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	bria := sc.perm(id["Bria, Riptide Rogue"], 0)
	sokka := sc.perm(id["Sokka, Tenacious Tactician"], 0)
	tok := sc.perm(ally, 0)
	sc.perm(id["Mountain"], 0)
	shock := sc.hand(id["Shock"], 0)
	for _, w := range []struct {
		name string
		o    Obj
		want int
	}{{"Bria", bria, 0}, {"Sokka", sokka, 1}, {"Ally token", tok, 2}} {
		if got := sc.s.prowessGrantCount(sc.k, sc.slot(w.o)); got != w.want {
			t.Errorf("%s: %d granted prowess, want %d", w.name, got, w.want)
		}
	}
	sc.do(t, AkCast, uint8(shock), 0xff)
	sc.do(t, AkTarget, 0xff, uint8(PlayerTarget1))
	sc.s.Step()
	per := map[Obj]int{}
	printed := map[Obj]int{}
	for i := 0; i < int(sc.s.npend); i++ {
		p := sc.s.pend[i]
		if p.ab == abProwess {
			per[p.obj]++
		} else {
			printed[p.obj]++
		}
	}
	if per[bria] != 0 || per[sokka] != 1 || per[tok] != 2 || len(per) != 2 {
		t.Errorf("abProwess per permanent %v, want sokka 1 / token 2 / bria 0", per)
	}
	if printed[bria] == 0 || printed[sokka] == 0 {
		t.Errorf("printed prowess triggers %v: a printed-prowess recipient lost its own trigger", printed)
	}
}
