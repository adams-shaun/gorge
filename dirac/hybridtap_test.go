package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHybridTapOrder: two stacked hand-paid hybrid casts (Teach by Example,
// {U/R}{U/R}, both pips answered R) tap the Mountains in battlefield entry
// order, not ObjID order -- gorge pays a hybrid cast through its manual
// CR 601.2g window (pay.ManaSourceIDs). Lockstep seed 459 of
// TestDelayspellLockstep diverged on the second cast's Mountains.
func TestHybridTapOrder(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Teach by Example", Count: 2},
		{Name: "Mountain", Count: 8},
	}
	delayspellSet := cardsettest.Set(t, "delayspell")
	tbl, da, _, err := delayspellSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	var m [4]Obj
	for i := range m {
		m[i] = sc.obj(id["Mountain"], 0)
	}
	// entry order m2, m3, m0, m1: ObjID order would tap m0, m1 first
	for _, i := range []int{2, 3, 0, 1} {
		sc.enterAs(m[i], 0)
	}
	castRR := func(spell Obj) {
		t.Helper()
		sc.do(t, AkCast, TCode(spell), scAny)
		for pip := 0; pip < 2; pip++ {
			d := sc.s.Step()
			if d.Kind != DkChoose || d.Sub != ChGenPip {
				t.Fatalf("pip %d: decision %v/%d", pip, d.Kind, d.Sub)
			}
			var pick Action
			found := false
			for i := 0; i < int(d.N); i++ {
				if a := d.Options[i]; a.Mode() == kop.R {
					pick, found = a, true
				}
			}
			if !found {
				t.Fatalf("pip %d: red not offered: %v", pip, d.Options[:d.N])
			}
			sc.s.Apply(pick)
		}
		sc.s.Step()
	}
	castRR(sc.hand(id["Teach by Example"], 0))
	for _, i := range []int{2, 3} {
		if !sc.tapped(m[i]) {
			t.Fatalf("first cast: Mountain %d (entered first) untapped", i)
		}
	}
	if sc.tapped(m[0]) || sc.tapped(m[1]) {
		t.Fatalf("first cast tapped a later-entered Mountain")
	}
	sc.passBoth(t)
	castRR(sc.hand(id["Teach by Example"], 0))
	for i := range m {
		if !sc.tapped(m[i]) {
			t.Fatalf("second cast: Mountain %d untapped", i)
		}
	}
}

// TestHybridTwobridFirstSource: a twobrid cast whose floating R pays two
// pips and one generic still owes one mana; gorge's manual CR 601.2g window
// taps the first untapped source in battlefield order (Dismal Backwater,
// floating U), not the planner's monocolour Mountain (key 4, flex).
func TestHybridTwobridFirstSource(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Dismal Backwater", Count: 1},
		{Name: "Bloodfell Caves", Count: 1},
		{Name: "Mountain", Count: 2},
	}
	fdnSet := cardsettest.Set(t, "fdn")
	tbl, da, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	dismal := sc.perm(id["Dismal Backwater"], 0)
	caves := sc.perm(id["Bloodfell Caves"], 0)
	m1 := sc.perm(id["Mountain"], 0)
	m2 := sc.perm(id["Mountain"], 0)
	for _, o := range []Obj{dismal, caves} {
		// the gain lands enter tapped
		sc.s.perms[sc.slot(o)].flags &^= pfTapped
	}
	sc.s.pl[0].pool[kop.R] = 3
	cost := kop.ManaCost{Generic: 2}
	cost.Pips[kop.R] = 2
	sc.s.tapForHyb(sc.k, 0, &cost, 0, NoObj, false)
	if !sc.tapped(dismal) || sc.tapped(caves) || sc.tapped(m1) || sc.tapped(m2) {
		t.Fatalf("taps: dismal=%v caves=%v m1=%v m2=%v, want only Dismal Backwater",
			sc.tapped(dismal), sc.tapped(caves), sc.tapped(m1), sc.tapped(m2))
	}
	if got, want := sc.s.pl[0].pool, [kop.NumManaKinds]uint8{kop.U: 1, kop.R: 3}; got != want {
		t.Fatalf("pool %v, want %v", got, want)
	}
}
