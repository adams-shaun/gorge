package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUnrelaxableFilterFundsItsFee: D'Vana Tendi's "{1}{G}{U}{U}" is cast
// from the hand over six Forests and Starfleet Academy's "{5}, {T}: Add X
// mana of any one color". No Forest makes {U}, and X is this board's
// creature-type count, above six, so the planner census cannot relax the
// ability (rules/potential_plan.go paymentPlanRelaxedAlternatives): the {5}
// must be funded from the Forests first (leaving one for the {G}) and the
// colour ask answered {U} through the prefix search (CR 605.3a, 602.2b,
// 601.2h). Before, the kernel funded no unrelaxable filter and the search
// spent its budget on Starfleet's free "{T}: Add {C}" branch, so the cast
// was not offered.
func TestUnrelaxableFilterFundsItsFee(t *testing.T) {
	sc, id := unrelaxedFilterScen(t)
	ttc := sc.perm(id["Three Tree City"], 0)
	explorer := sc.perm(id["Saurian Explorer"], 0)
	for i := 0; i < 8; i++ {
		sc.perm(id["Grizzly Bears"], 0)
	}
	for i := 0; i < 2; i++ {
		sc.perm(id["Forest"], 0)
	}
	k, sl := sc.k, sc.slot(ttc)
	// Three Tree City's as-enters type choice: all eight Bears are the chosen
	// type, so its filter makes eight mana.
	bear := uint8(255)
	for i, name := range k.t.Subtypes {
		if name == "Bear" {
			bear = uint8(i)
		}
	}
	if bear == 255 {
		t.Fatal("the table has no Bear subtype")
	}
	sc.s.perms[sl].chosen = bear
	gaps := sc.s.scriptFilt(k, 0)
	if !gaps.has(sl) {
		t.Fatalf("Three Tree City is not a census filter gap: %v", gaps)
	}
	if n := sc.s.filtUnits(k, sl); n <= 6 {
		t.Fatalf("Three Tree City makes %d mana; the scenario needs an unrelaxable amount above six", n)
	}
	if sc.s.filtRelaxable(k, gaps) {
		t.Fatal("the scenario's gaps are relaxable; it is not the unrelaxable-gap case")
	}
	// Saurian Explorer's "{6}" is the play: two Forests and Three Tree
	// City's free "{T}: Add {C}" make three, so only the filter can pay it.
	ab := -1
	explorerCard := sc.s.objs[explorer].card
	for i := range k.t.Cards[explorerCard].Abilities {
		if k.t.Cards[explorerCard].Abilities[i].Kind == kop.AbActivated {
			ab = i
			break
		}
	}
	if ab < 0 {
		t.Fatal("Saurian Explorer has no activated ability in the table")
	}
	play := MakeAction(AkActivate, uint8(explorer), 0, uint8(ab))
	if !sc.s.filtScript(k, play, 0, false) {
		t.Fatalf("the filter search found no script for Saurian Explorer's {6} (fee %d, units %d)", sc.s.filtFee(k, sl), sc.s.filtUnits(k, sl))
	}
}

// unrelaxedFilterScen builds the dynfilter pool's scenario without the
// Plains library newSetScenTweak requires (the pool has none): an Island
// library, nothing in hand.
func unrelaxedFilterScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "dynfilter")
	deck := []cardset.Entry{
		{Name: "Starfleet Academy", Count: 1},
		{Name: "Three Tree City", Count: 1},
		{Name: "Saurian Explorer", Count: 1},
		{Name: "D'Vana Tendi, Optimistic Ensign", Count: 1},
		{Name: "Xenobotanist", Count: 1},
		{Name: "Llanowar Elves", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Island", Count: 6},
		{Name: "Forest", Count: 6},
		{Name: "Mountain", Count: 6},
	}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in the dynfilter table", e.Name)
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
	return newScenTableLib(t, tbl, id["Island"], Config{NoElide: true}), id
}
