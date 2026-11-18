package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 106.6: Somberwald Sage's creature-spell-only mana can't pay the {1} of
// Mossfire Valley's activated mana ability. Forest, Island, Mossfire Valley and
// the Sage make six mana at most (the Sage's three only for creature spells),
// so the seven-mana Axebane Stag is not castable; with a second Forest it is.
func TestRestrictedSageDoesNotFundFilterFee(t *testing.T) {
	names := []string{"Axebane Stag", "Mossfire Valley", "Somberwald Sage", "Island", "Jiang Yanggu, Wildcrafter"}
	set := cardsettest.Set(t, "restrfilterfee")
	var deck []cardset.Entry
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not in the restrfilterfee table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	deck = append(deck, cardset.Entry{Name: "Forest", Count: 1})
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, extra := range []bool{false, true} {
		sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
		sc.perm(id["Forest"], 0)
		sc.perm(id["Island"], 0)
		sc.perm(id["Mossfire Valley"], 0)
		sage := sc.perm(id["Somberwald Sage"], 0)
		// Jiang Yanggu grants the Sage (it wears a +1/+1 counter) an
		// unrestricted {T}: add one mana, an alternative to its three
		sc.perm(id["Jiang Yanggu, Wildcrafter"], 0)
		sc.s.perms[sc.slot(sage)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
		if extra {
			sc.perm(id["Forest"], 0)
		}
		stag := sc.hand(id["Axebane Stag"], 0)
		d := sc.s.Step()
		if got := castOffered(&d, stag); got != extra {
			t.Errorf("extra Forest=%v: cast offered=%v, want %v", extra, got, extra)
		}
	}
}
