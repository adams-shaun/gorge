package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFeeOrderPricesACastBesideAnotherAbilitySource: four Islands, Castle Doom
// (a plain {C} beside its artifact-only any-colour mana) and Baxter Building
// ("{4}, {T}: Add four mana in any combination of colors"). Mole Man's {2}{G}
// is payable only through Baxter's filter, funded by the four Islands. gorge's
// plain search spends its 160 nodes on Castle Doom's asks and finds nothing;
// its phased run in the fee order (covered funding, then the fee gaps) finds
// the script and offers the cast (CR 605.3a, 601.2g).
func TestFeeOrderPricesACastBesideAnotherAbilitySource(t *testing.T) {
	deck := []cardset.Entry{{Name: "Mole Man, Moloid Master", Count: 1}, {Name: "Baxter Building", Count: 1},
		{Name: "Castle Doom", Count: 1}, {Name: "Island", Count: 8}}
	tbl, _, _, err := cardsettest.Synth(t, "std-msh").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	for _, n := range []string{"Island", "Island", "Island", "Island", "Castle Doom", "Baxter Building"} {
		sc.perm(id[n], 0)
	}
	h := sc.hand(id["Mole Man, Moloid Master"], 0)
	if !sc.s.filtFeeGaps(sc.k, sc.s.scriptFilt(sc.k, 0)).any() {
		t.Fatal("Baxter Building is no fee gap: the fee order does not run")
	}
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && Obj(a.Source()) == h {
			return
		}
	}
	t.Fatal("Mole Man's cast is not offered over Baxter Building's filter funded by four Islands")
}
