package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

func vesselScen(t *testing.T, board ...string) (sc *scen, hydra Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Bioessence Hydra", Count: 1}, {Name: "Forest", Count: 10}}
	seen := map[string]bool{}
	for _, n := range board {
		if !seen[n] {
			seen[n] = true
			deck = append(deck, cardset.Entry{Name: n, Count: 1})
		}
	}
	set := cardsettest.Synth(t, "tail-43")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	for _, n := range board {
		x := sc.slot(sc.perm(id[n], 0))
		sc.s.perms[x].flags &^= pfTapped
		if n == "Krosan Drover" {
			sc.s.perms[x].flags |= pfSick
		}
	}
	return sc, sc.hand(id["Bioessence Hydra"], 0)
}

// TestTapXSearchTapsAVesselOnce: Bioessence Hydra ({3}{G}{U}) beside an Island,
// a Firemind Vessel and a Survivors' Encampment with a creature to tap is four
// mana at most, so it is not offered (CR 605.3b, 106.1: each source taps once).
// The tap-another search once re-activated the Vessel at every node (the
// AnyDifferent mask is the root's), floating W and U until the pool paid. With
// a second Island the same board makes five and the play is offered.
func TestTapXSearchTapsAVesselOnce(t *testing.T) {
	short, hydra := vesselScen(t, "Island", "Firemind Vessel", "Survivors' Encampment", "Krosan Drover")
	if !short.k.tapMana || !short.s.scriptTapX(short.k, 0).any() || short.s.anyDifferentGaps(short.k, 0).count() != 1 {
		t.Fatal("precondition: the board has a live tap-another gap and one AnyDifferent gap")
	}
	d := short.s.Step()
	if castOffered(&d, hydra) {
		t.Fatal("Bioessence Hydra offered with four mana: the Firemind Vessel was tapped more than once")
	}
	full, hydra := vesselScen(t, "Island", "Island", "Firemind Vessel", "Survivors' Encampment", "Krosan Drover")
	d = full.s.Step()
	if !castOffered(&d, hydra) {
		t.Fatal("Bioessence Hydra not offered with five mana: Island, Island, Vessel's two, Encampment's one")
	}
}
