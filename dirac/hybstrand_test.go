package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHybridFaceCountsRestrictedSource: Moonrise Cleric ({1}{W/B}{W/B}) is offered
// on the Village's restricted {W}, and the hybrid faces count that mana as the
// cast window does (CR 106.6, 601.2f), so the {W} face is asked and the cast
// goes through to the stack rather than reversing (CR 733.1).
func TestHybridFaceCountsRestrictedSource(t *testing.T) {
	set := cardsettest.Set(t, "blb")
	var deck []cardset.Entry
	for _, n := range []string{"Moonrise Cleric", "Swamp", "Hidden Grotto", "Lupinflower Village"} {
		deck = append(deck, cardset.Entry{Name: n, Count: 10})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	for _, n := range []string{"Swamp", "Hidden Grotto", "Lupinflower Village"} {
		sc.perm(id[n], 0)
	}
	o := sc.hand(id["Moonrise Cleric"], 0)
	d := sc.s.Step()
	if !castOffered(&d, o) {
		t.Fatal("Moonrise Cleric is not offered")
	}
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
			sc.s.Apply(a)
			break
		}
	}
	for d = sc.s.Step(); d.Kind == DkChoose; d = sc.s.Step() {
		sc.s.Apply(d.Options[0])
	}
	if z := sc.s.objs[o].zone; z != kop.ZoneStack {
		t.Fatalf("Moonrise Cleric is in zone %v, want the stack", z)
	}
}
