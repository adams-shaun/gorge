package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

func nightscapeOffers(t *testing.T, board ...string) map[int]bool {
	t.Helper()
	deck := []cardset.Entry{{Name: "Nightscape Apprentice", Count: 1}, {Name: "Shimmering Grotto", Count: 1},
		{Name: "Carrier Thrall", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}}
	set := cardsettest.Synth(t, "tail-10")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	for c := range tbl.Cards {
		if tbl.Cards[c].Name == "c_1_1_eldrazi_scion_sac" {
			id["Eldrazi Scion Token"] = CardID(c)
		}
	}
	var na Obj
	for _, n := range board {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the table", n)
		}
		o := sc.perm(c, 0)
		if n == "Eldrazi Scion Token" {
			sc.s.objs[o].of |= ofToken
		}
		if n == "Nightscape Apprentice" {
			na = o
		}
	}
	got := map[int]bool{}
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkActivate && Obj(a.Source()) == na {
			got[int(a.Mode())] = true
		}
	}
	return got
}

// TestNightscapeGrottoOffers: the control, Grotto's filter makes the U and
// R the two activations need with no Scion beside it.
func TestNightscapeGrottoOffers(t *testing.T) {
	got := nightscapeOffers(t, "Nightscape Apprentice", "Swamp", "Swamp", "Swamp", "Plains", "Plains", "Shimmering Grotto")
	if !got[0] || !got[1] {
		t.Fatalf("precondition: offers %v", got)
	}
}

// TestNightscapeGrottoScionOffers: an Eldrazi Scion beside the Grotto is no
// census gap (mana_filter.go gapOthers), so both activations stay offered.
func TestNightscapeGrottoScionOffers(t *testing.T) {
	got := nightscapeOffers(t, "Nightscape Apprentice", "Swamp", "Swamp", "Swamp", "Plains", "Plains", "Shimmering Grotto", "Eldrazi Scion Token")
	if !got[0] || !got[1] {
		t.Fatalf("offers %v", got)
	}
}
