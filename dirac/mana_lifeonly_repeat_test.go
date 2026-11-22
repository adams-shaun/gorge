package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// CR 605.1a, 601.2h, 119.4: Treasonous Ogre's "Pay 3 life: Add {R}" taps nothing,
// so gorge's prefix search activates it once and prices the play over the
// floating {R} with the unit still in the census: Wand of Orcus's Equip {3}
// beside one Plains is offered while two payments (6 life) leave the payer
// above 0 life, and not at 6 life.
func TestLifeOnlyManaPaysEquipRepeatedly(t *testing.T) {
	deck := []cardset.Entry{{Name: "Wand of Orcus", Count: 1}, {Name: "Prismite", Count: 1}, {Name: "Treasonous Ogre", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8}}
	tbl, _, _, err := cardsettest.Synth(t, "tail-51").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	for _, tc := range []struct {
		life  int
		lands int
		want  bool
	}{{20, 1, true}, {7, 1, true}, {6, 1, false}} {
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true, GorgeLowering: true, GorgeAutopaySurface: true})
		for i := 0; i < tc.lands; i++ {
			sc.perm(id["Plains"], 0)
		}
		sc.perm(id["Prismite"], 0)
		wand := sc.perm(id["Wand of Orcus"], 0)
		ogre := sc.perm(id["Treasonous Ogre"], 0)
		if !lifeOnlyPain(sc.s.manaPCAt(sc.k, sc.slot(ogre))) {
			t.Fatal("precondition: the Ogre is not the life-only pain census unit")
		}
		sc.s.pl[0].life = int16(tc.life)
		got := false
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(wand) {
				got = true
			}
		}
		t.Logf("life %d lands %d: offered %v", tc.life, tc.lands, got)
		if got != tc.want {
			t.Errorf("life %d, %d Plains: equip offered = %v, want %v", tc.life, tc.lands, got, tc.want)
		}
	}
}
