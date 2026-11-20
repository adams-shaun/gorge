package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

func comboDualsOffers(t *testing.T, lands []string, spell string) bool {
	t.Helper()
	set := cardsettest.Synth(t, "std-msh")
	deck := []cardset.Entry{{Name: spell, Count: 1}}
	seen := map[string]bool{}
	for _, l := range lands {
		if !seen[l] {
			deck = append(deck, cardset.Entry{Name: l, Count: 1})
			seen[l] = true
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, da[0], Config{})
	for _, l := range lands {
		sc.perm(id[l], 0)
	}
	sp := sc.hand(id[spell], 0)
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(sp) {
			return true
		}
	}
	return false
}

func TestFilterFeeOrderFindsBaxterWithDualLands(t *testing.T) {
	lands := []string{"Mountain", "Hell's Kitchen", "Fisk Tower", "Baxter Building", "Dark Fortress"}
	if !comboDualsOffers(t, lands, "Serpent Specialist") {
		t.Fatal("Serpent Specialist ({G}) is not offered: Baxter Building's {4},{T} makes G from the four other lands")
	}
}
