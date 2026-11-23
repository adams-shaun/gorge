package dirac

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// niallHengeScen is the divergence board (tail-26 pair 66, decision 633) over
// the tail-26 table: Niall Silvain, three Forests, two Islands, Henge of
// Ramos, A-Tyvar Kell (Elves have "{T}: Add {B}") and four Elf Warrior tokens.
func niallHengeScen(t *testing.T, tyvar bool, elves int) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Niall Silvain", Count: 1}, {Name: "A-Tyvar Kell", Count: 1}, {Name: "Henge of Ramos", Count: 1},
		{Name: "Forest", Count: 3}, {Name: "Island", Count: 2},
	}
	tbl, da, _, err := cardsettest.Synth(t, "tail-26").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for c := range tbl.Cards {
		if strings.Contains(tbl.Cards[c].Name, "elf_warrior") {
			id["elf"] = CardID(c)
		}
	}
	if _, ok := id["elf"]; !ok {
		t.Fatal("the Elf Warrior token is not in the matchup table")
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	sc.s.cfg |= cfgGorgeAutopay | cfgGorgeUnpriced
	for i := 0; i < 3; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Henge of Ramos"], 0)
	if tyvar {
		sc.perm(id["A-Tyvar Kell"], 0)
	}
	for i := 0; i < elves; i++ {
		o := sc.perm(id["elf"], 0)
		sc.s.objs[o].of |= ofToken
	}
	return sc, sc.perm(id["Niall Silvain"], 0)
}

// CR 602.2, 605.3: Niall Silvain's {G}{G}{G}{G},{T} is payable from three
// Forests and Henge of Ramos's "{2},{T}: one mana of any color" funded by the
// two Islands; the granted Elf taps ({B}) beside it change nothing. gorge's
// planner offers the activation (tail-26 pair 66, decision 633).
func TestNiallSilvainRegenerateOfferedBesideHengeAndElves(t *testing.T) {
	for _, c := range []struct {
		name  string
		tyvar bool
		elves int
	}{{"board", true, 4}, {"no-elves", true, 0}, {"no-tyvar", false, 4}, {"bare", false, 0}, {"one-elf", true, 1}, {"two-elves", true, 2}, {"three-elves", true, 3}} {
		t.Run(c.name, func(t *testing.T) { niallOffered(t, c.tyvar, c.elves) })
	}
}

func niallOffered(t *testing.T, tyvar bool, elves int) {
	sc, niall := niallHengeScen(t, tyvar, elves)
	d := sc.s.Step()
	var offered bool
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(niall) {
			offered = true
		}
	}
	if !offered {
		t.Fatalf("Niall Silvain's activation is not offered; options %v", d.Options[:d.N])
	}
}
