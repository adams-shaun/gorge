package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestPlanKey8OverEveryTie: {4}{U} from three Forests, six Islands, two
// Plains and two gain lands ties on keys 1-7 for more count vectors than
// the planner used to keep (eight), and the dropped ones held key 8's
// least ObjID sequence: gorge taps Forest, Forest, Island, Island, Forest
// (pair 517's board; 517 and 17 more in the 10k slice). Objects are made
// in ObjID order and enter in pair 517's battlefield order.
func TestPlanKey8OverEveryTie(t *testing.T) {
	deck := []cardset.Entry{{Name: "Blossoming Sands", Count: 1}, {Name: "Tranquil Cove", Count: 1}, {Name: "Forest", Count: 3}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 2}}
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
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	// pair 517's ObjIDs: Sands 6, Forests 11 14 15, Islands 18-23, Plains
	// 29 30, Cove 37
	byID := []string{"Blossoming Sands", "Forest", "Forest", "Forest", "Island", "Island", "Island", "Island", "Island", "Island", "Plains", "Plains", "Tranquil Cove"}
	o := make([]Obj, len(byID))
	for i, n := range byID {
		o[i] = sc.obj(id[n], 0)
	}
	sands, f11, f14, f15 := o[0], o[1], o[2], o[3]
	i18, i19, i20, i21, i22, i23 := o[4], o[5], o[6], o[7], o[8], o[9]
	p29, p30, cove := o[10], o[11], o[12]
	for _, x := range []Obj{f14, i21, i20, p30, f15, i22, i19, i18, i23, p29, f11, sands, cove} {
		sc.enterAs(x, 0)
		sc.s.perms[sc.slot(x)].flags &^= pfTapped // the gain lands entered long ago
	}
	sc.s.npend = 0
	c := cost(4, kop.U)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	want := map[Obj]bool{f14: true, f15: true, i18: true, i23: true, f11: true}
	for _, x := range o {
		if sc.tapped(x) != want[x] {
			t.Errorf("object %d (%s) tapped=%v, want %v", x, tbl.Cards[sc.s.objs[x].card].Name, sc.tapped(x), want[x])
		}
	}
}

// TestPlanDynamicAmountUnitLast: gorge's planner census lists a source
// whose amount it evaluates (Elvish Archdruid's {G} for each Elf) after
// every fixed and choice-shaped source (payment_plan.go
// paymentPlanManaUnitsOnlyCompute, the evaluated-amount layer), so key 8
// compares the witness Island, Swamp, Archdruid in that order and keeps
// the earlier Island: with Erudite Wizard's {U} demand in hand, {2}{G}
// taps the first Island, the Swamp and the Archdruid (pair 1891). Before, the kernel listed the Archdruid in
// battlefield order and its lower ObjID led the sequence, so the second
// Island was tapped.
func TestPlanDynamicAmountUnitLast(t *testing.T) {
	deck := []cardset.Entry{{Name: "Elvish Archdruid", Count: 1}, {Name: "Erudite Wizard", Count: 1}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 5}}
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
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	druid := sc.obj(id["Elvish Archdruid"], 0)
	i1 := sc.obj(id["Island"], 0)
	i2 := sc.obj(id["Island"], 0)
	sw := sc.obj(id["Swamp"], 0)
	sc.hand(id["Erudite Wizard"], 0)
	for _, x := range []Obj{i1, druid, i2, sw} {
		sc.enterAs(x, 0)
	}
	sc.s.npend = 0
	c := cost(2, kop.G)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	want := map[Obj]bool{i1: true, sw: true, druid: true}
	for _, x := range []Obj{druid, i1, i2, sw} {
		if sc.tapped(x) != want[x] {
			t.Errorf("object %d (%s) tapped=%v, want %v", x, tbl.Cards[sc.s.objs[x].card].Name, sc.tapped(x), want[x])
		}
	}
}

// TestPlanKey8ComparesProduced: gorge's key 8 compares the witness step by
// step, the source's ObjID first and then the mana it produces (WUBRGC
// vector, so {U} before {W}). {2}{W} from pair 8715's board (Squad
// Rallier): Island, Tranquil Cove, Dismal Backwater ties keys 1-7 with
// Island, Tranquil Cove, Scoured Barrens, and the ObjID sequences differ
// only at the third step (Dismal Backwater's is older), but at the second
// the Cove makes {W} in the first and {U} in the second: gorge taps the
// Barrens.
func TestPlanKey8ComparesProduced(t *testing.T) {
	deck := []cardset.Entry{{Name: "Tranquil Cove", Count: 2}, {Name: "Swiftwater Cliffs", Count: 1}, {Name: "Dismal Backwater", Count: 1}, {Name: "Thornwood Falls", Count: 1}, {Name: "Scoured Barrens", Count: 1}, {Name: "Island", Count: 4}, {Name: "Inspiration from Beyond", Count: 1}, {Name: "Treetop Snarespinner", Count: 1}}
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
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	// pair 8715's ObjIDs: Dismal 45, Island 58, Barrens 66, Swiftwater 70,
	// Thornwood 72, Coves 74 75
	byID := []string{"Dismal Backwater", "Island", "Scoured Barrens", "Swiftwater Cliffs", "Thornwood Falls", "Tranquil Cove", "Tranquil Cove"}
	o := make([]Obj, len(byID))
	for i, n := range byID {
		o[i] = sc.obj(id[n], 0)
	}
	dismal, isl, barrens, swift, thorn, c74, c75 := o[0], o[1], o[2], o[3], o[4], o[5], o[6]
	for _, x := range []Obj{c75, c74, swift, dismal, isl, thorn, barrens} {
		sc.enterAs(x, 0)
		sc.s.perms[sc.slot(x)].flags &^= pfTapped
	}
	sc.s.npend = 0
	sc.hand(id["Inspiration from Beyond"], 0)
	sc.hand(id["Treetop Snarespinner"], 0)
	c := cost(2, kop.W)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	want := map[Obj]bool{isl: true, c74: true, barrens: true}
	for _, x := range o {
		if sc.tapped(x) != want[x] {
			t.Errorf("object %d (%s) tapped=%v, want %v", x, tbl.Cards[sc.s.objs[x].card].Name, sc.tapped(x), want[x])
		}
	}
}
