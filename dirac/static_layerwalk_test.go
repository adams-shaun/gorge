package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 613 layer scenarios (layers 4, 5, 7b in timestamp and
// dependency order; gorge chars.Compute / TypesAndLandSubtypeSet).

type l3scen struct {
	sc   *scen
	id   map[string]CardID
	pt32 int // the one-shot 3/2 set-P/T override record's index
	add6 int // the one-shot "is also a 3/3 creature" record's index
}

func newL3(t *testing.T) *l3scen {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1},
		{Name: "Ambush Wolf", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	l := &l3scen{id: map[string]CardID{}, pt32: -1, add6: -1}
	for _, c := range da {
		l.id[tbl.Cards[c].Name] = c
	}
	for i, o := range tbl.Overrides {
		if o.HasPT && o.SetP == 3 && o.SetT == 2 && !o.HasTypes && !o.LoseAbilities {
			l.pt32 = i
		}
		if o.HasPT && o.SetP == 3 && o.SetT == 3 && o.AddTypes == kop.TCreature && !o.HasTypes {
			l.add6 = i
		}
	}
	if l.pt32 < 0 || l.add6 < 0 {
		t.Skip("the one-shot override records are not in this matchup's table")
	}
	l.sc = newScenTable(t, tbl, Config{NoElide: true})
	if l.sc.k.gen.fxNeed == 0 {
		t.Fatal("an FDN table with override statics leaves fxNeed empty")
	}
	return l
}

func (l *l3scen) aura(name string, host Obj) {
	a := l.sc.perm(l.id[name], 1)
	l.sc.s.attachTo(l.sc.k, l.sc.slot(a), l.sc.slot(host))
	l.sc.s.refreshTy(l.sc.k)
}

func (l *l3scen) record(idx int, host Obj) {
	l.sc.s.applyOverride(l.sc.k, l.sc.slot(host), idx, l.id["Ambush Wolf"], false)
	l.sc.s.refreshTy(l.sc.k)
}

// TestL3WitnessProtectionAndImprisonedInTheMoon: either order, the creature
// is a colourless land (gorge, CR 613.8: Witness Protection's
// "enchanted creature" depends on Imprisoned in the Moon, which applies
// first and stops it applying). The fixed order made it a 1/1 green-white
// creature (the audit's dirac-only error).
func TestL3WitnessProtectionAndImprisonedInTheMoon(t *testing.T) {
	for _, order := range [][2]string{{"Witness Protection", "Imprisoned in the Moon"}, {"Imprisoned in the Moon", "Witness Protection"}} {
		l := newL3(t)
		wolf := l.sc.perm(l.id["Ambush Wolf"], 0)
		l.aura(order[0], wolf)
		l.aura(order[1], wolf)
		sl := l.sc.slot(wolf)
		k := l.sc.k
		if ty := l.sc.s.typesAt(k, sl) &^ kop.TToken; ty&^(kop.TLegendary|kop.TBasic) != kop.TLand {
			t.Errorf("%v: types %#x, want a land", order, ty)
		}
		if c := l.sc.s.colorsAt(k, sl); c&^kop.ColC != 0 {
			t.Errorf("%v: colours %#x, want colourless", order, c)
		}
	}
}

// TestL3TwoBasePTSetters: an Aura's static and a one-shot set P/T on one
// creature, both arrival orders: the newer wins (CR 613.7, layer 7b).
func TestL3TwoBasePTSetters(t *testing.T) {
	l := newL3(t)
	wolf := l.sc.perm(l.id["Ambush Wolf"], 0)
	l.aura("Witness Protection", wolf)
	l.record(l.pt32, wolf)
	if p, tt := l.sc.s.basePTGen(l.sc.k, l.sc.slot(wolf)); p != 3 || tt != 2 {
		t.Errorf("aura then one-shot: base %d/%d, want 3/2", p, tt)
	}
	l = newL3(t)
	wolf = l.sc.perm(l.id["Ambush Wolf"], 0)
	l.record(l.pt32, wolf)
	l.aura("Witness Protection", wolf)
	if p, tt := l.sc.s.basePTGen(l.sc.k, l.sc.slot(wolf)); p != 1 || tt != 1 {
		t.Errorf("one-shot then aura: base %d/%d, want 1/1", p, tt)
	}
}

// TestL3TypeSetterThenAdd: a type setter (Imprisoned in the Moon: a land)
// and a one-shot "is also a 3/3 creature", both orders (layer 4 in
// timestamp order: set then add keeps the add; add then set removes it).
func TestL3TypeSetterThenAdd(t *testing.T) {
	l := newL3(t)
	wolf := l.sc.perm(l.id["Ambush Wolf"], 0)
	l.aura("Imprisoned in the Moon", wolf)
	l.record(l.add6, wolf)
	sl := l.sc.slot(wolf)
	if ty := l.sc.s.typesAt(l.sc.k, sl); ty&(kop.TLand|kop.TCreature) != kop.TLand|kop.TCreature {
		t.Errorf("set then add: types %#x, want a land creature", ty)
	}
	if p, tt := l.sc.s.basePTGen(l.sc.k, sl); p != 3 || tt != 3 {
		t.Errorf("set then add: base %d/%d, want 3/3", p, tt)
	}
	l = newL3(t)
	wolf = l.sc.perm(l.id["Ambush Wolf"], 0)
	l.record(l.add6, wolf)
	l.aura("Imprisoned in the Moon", wolf)
	if ty := l.sc.s.typesAt(l.sc.k, l.sc.slot(wolf)); ty&kop.TCreature != 0 || ty&kop.TLand == 0 {
		t.Errorf("add then set: types %#x, want a non-creature land", ty)
	}
}
