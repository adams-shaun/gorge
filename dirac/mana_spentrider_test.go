package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestManaHasteRiderMatchesTheSpell: Carnelian Orb of Dragonkind's rider
// (Spell.Dragon+Creature) reads the spell as cast, not the printed card. A
// Dragon cast face down is a typeless 2/2 with no creature type (CR 708.4;
// gorge state.Object.FaceDownHidden) and gains nothing; a changeling creature
// spell is a Dragon (CR 702.73a; gorge IntrinsicAllCreatureTypes) and gains
// haste. The oracle's face-down cast pays from floating mana only, so the
// lockstep cannot pay one with the Orb's tap: this pins the matcher.
func TestManaHasteRiderMatchesTheSpell(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Carnelian Orb of Dragonkind", Count: 1}, {Name: "Acid-Spewer Dragon", Count: 1},
		{Name: "Barkform Harvester", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Mountain", Count: 36},
	}
	for _, tc := range []struct {
		card     string
		faceDown bool
		want     bool
	}{
		{"Acid-Spewer Dragon", false, true}, // the precondition: the printed card matches
		{"Acid-Spewer Dragon", true, false},
		{"Barkform Harvester", false, true},
		{"Hill Giant", false, false},
	} {
		sc, id := newManaspentScen(t, deck)
		k := sc.k
		if k.gen == nil || !k.gen.anyFaceDown {
			t.Fatal("the manaspent pool has no face-down mechanic")
		}
		orb := sc.perm(id["Carnelian Orb of Dragonkind"], 0)
		c := id[tc.card]
		o := sc.obj(c, 0)
		sc.s.objs[o].zone = kop.ZoneStack
		e := stackEntry{kind: seSpell, obj: o, card: c, ab: 255, ctrl: 0}
		if tc.faceDown {
			sc.s.objs[o].slot = slotFaceDownSpell // as pushBuild marks a face-down cast
			e.mode = seFaceDown | kop.MorphMegamorph
		}
		sc.s.push(e)
		b := sc.s.bld()
		b.obsSl, b.bfl, b.e.obj = sc.slot(orb)+1, bflPushed, o
		sc.s.manaHasteRider(k)
		if got := sc.s.stack[sc.s.sp-1].remVer&spellManaHaste != 0; got != tc.want {
			t.Errorf("%s (face down %v): haste stamp %v, want %v", tc.card, tc.faceDown, got, tc.want)
		}
	}
}

// newManaspentScen: deck's matchup table on the manaspent pool, seat 0 to act
// in its first main phase with empty hands (as newKfacedownScen).
func newManaspentScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	tbl, da, _, err := cardsettest.Set(t, "manaspent").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	s := new(State)
	s.Init(tbl, da, da, 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}
