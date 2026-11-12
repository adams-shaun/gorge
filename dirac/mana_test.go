package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

func (sc *scen) tapped(o Obj) bool { return sc.s.perms[sc.slot(o)].flags&pfTapped != 0 }

// enterAs puts pre-allocated object o onto the battlefield (entry order is
// call order, independent of object ids).
func (sc *scen) enterAs(o Obj, p uint8) {
	sl := sc.s.enter(sc.k, o, p, false, kop.ZoneNone)
	sc.s.perms[sl].flags &^= pfSick
}

func cost(generic int, pips ...int) kop.ManaCost {
	var m kop.ManaCost
	m.Generic = uint8(generic)
	for _, p := range pips {
		m.Pips[p]++
	}
	return m
}

// The planner's key-8 tie-break: among tied plans, the least ObjID sequence
// read in unit (battlefield entry) order. gorge's worked example: zone order
// Mountain#40, Mountain#12, Forest#33 paying {1}{R} taps Mountain#12 and
// Forest#33.
func TestAutopayLexTieBreak(t *testing.T) {
	sc := newScen(t, Config{})
	m12 := sc.obj(v.Mountain, 0)
	f33 := sc.obj(v.Forest, 0)
	m40 := sc.obj(v.Mountain, 0)
	sc.enterAs(m40, 0)
	sc.enterAs(m12, 0)
	sc.enterAs(f33, 0)
	c := cost(1, kop.R)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if !sc.tapped(m12) || !sc.tapped(f33) || sc.tapped(m40) {
		t.Fatalf("tapped m12=%v f33=%v m40=%v; want m12+f33", sc.tapped(m12), sc.tapped(f33), sc.tapped(m40))
	}
	if sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("pool not empty: %v", sc.s.pl[0].pool)
	}
}

// The census holds every source, not the first 48: with 60 Forests, the key-8
// tie-break sees the least ObjID even when it entered last. f[0] is allocated
// first (the least ObjID) but enters at position 60. gorge's uncapped census
// (paymentPlanManaUnitsOnlyCompute) offers it to the same tie-break, so the
// kernel must tap it.
func TestAutopayCensusPastFortyEight(t *testing.T) {
	sc := newScen(t, Config{})
	f := make([]Obj, 60)
	for i := range f {
		f[i] = sc.obj(v.Forest, 0)
	}
	for i := 1; i < len(f); i++ {
		sc.enterAs(f[i], 0)
	}
	sc.enterAs(f[0], 0)
	if n := sc.s.ctrlM[0].count(); n != 60 || n <= 48 {
		t.Fatalf("%d Forests on the battlefield, want 60 (more than 48)", n)
	}
	c := cost(1)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if !sc.tapped(f[0]) {
		t.Fatalf("the least-ObjID Forest did not tap")
	}
	for i := 1; i < len(f); i++ {
		if sc.tapped(f[i]) {
			t.Fatalf("Forest %d (entry position %d) tapped beside f[0]", i, i)
		}
	}
}

// HandReserve (key 6): a {G}{G} card in hand keeps the Forest untapped.
func TestAutopayHandReserve(t *testing.T) {
	sc := newScen(t, Config{})
	m12 := sc.obj(v.Mountain, 0)
	f33 := sc.obj(v.Forest, 0)
	m40 := sc.obj(v.Mountain, 0)
	sc.enterAs(m40, 0)
	sc.enterAs(m12, 0)
	sc.enterAs(f33, 0)
	sc.hand(v.ColossalDreadmaw, 0) // {4}{G}{G}
	c := cost(1, kop.R)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if !sc.tapped(m12) || !sc.tapped(m40) || sc.tapped(f33) {
		t.Fatalf("tapped m12=%v m40=%v f33=%v; want both Mountains", sc.tapped(m12), sc.tapped(m40), sc.tapped(f33))
	}
}

// Key 2: lands before creature sources.
func TestAutopayLandsBeforeDorks(t *testing.T) {
	sc := newScen(t, Config{})
	elf := sc.perm(v.LlanowarElves, 0)
	f := sc.perm(v.Forest, 0)
	m := sc.perm(v.Mountain, 0)
	c := cost(1, kop.G)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if sc.tapped(elf) || !sc.tapped(f) || !sc.tapped(m) {
		t.Fatalf("elf=%v forest=%v mountain=%v; want lands only", sc.tapped(elf), sc.tapped(f), sc.tapped(m))
	}
}

// The floating pool pays first; nothing is tapped when it covers the cost.
func TestAutopayPoolFirst(t *testing.T) {
	sc := newScen(t, Config{})
	m := sc.perm(v.Mountain, 0)
	sc.s.pl[0].pool[kop.R] = 1
	c := cost(0, kop.R)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if sc.tapped(m) || sc.s.pl[0].pool[kop.R] != 0 {
		t.Fatalf("pool should pay {R}")
	}
	// pool R + one land pays {1}{R}: R from the pool, generic from the
	// Mountain (keys 2-7 tie; the Mountain's ObjID is lower)
	f := sc.perm(v.Forest, 0)
	sc.s.pl[0].pool[kop.R] = 1
	c = cost(1, kop.R)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if !sc.tapped(m) || sc.tapped(f) {
		t.Fatalf("mountain=%v forest=%v; want the Mountain (lex least)", sc.tapped(m), sc.tapped(f))
	}
}

// Generic is spent from the pool in order C W U B R G.
func TestPayFromPoolGenericOrder(t *testing.T) {
	sc := newScen(t, Config{})
	pl := &sc.s.pl[0]
	pl.pool = [kop.NumManaKinds]uint8{kop.W: 1, kop.G: 1, kop.C: 1}
	c := cost(2)
	sc.s.payFromPool(0, &c, 0)
	if pl.pool[kop.G] != 1 || pl.pool[kop.C] != 0 || pl.pool[kop.W] != 0 {
		t.Fatalf("pool after {2}: %v; want only G left", pl.pool)
	}
}

func TestAffordsHall(t *testing.T) {
	sc := newScen(t, Config{})
	sc.perm(v.Mountain, 0)
	sc.perm(v.Mountain, 0)
	sc.perm(v.Forest, 0)
	var av avail
	sc.s.availFor(sc.k, 0, &av)
	for _, tc := range []struct {
		c  kop.ManaCost
		ok bool
	}{
		{cost(1, kop.R), true},
		{cost(0, kop.R, kop.R, kop.G), true},
		{cost(0, kop.G, kop.G), false},
		{cost(3), true},
		{cost(4), false},
		{cost(1, kop.U), false},
	} {
		if got := av.affords(&tc.c, pipMaskOf(&tc.c), 0); got != tc.ok {
			t.Errorf("affords %+v = %v want %v", tc.c, got, tc.ok)
		}
	}
}

// A tap-and-sacrifice source (Treasure) is a last resort: unused while
// tap-only sources can pay, used (and sacrificed) when they can't.
func TestAutopayTreasureLastResort(t *testing.T) {
	sc := newScen(t, Config{})
	f := sc.perm(v.Forest, 0)
	tr := sc.obj(v.TreasureToken, 0)
	sc.s.objs[tr].of |= ofToken
	sc.enterAs(tr, 0)
	c := cost(0, kop.G)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if !sc.tapped(f) || !sc.onBF(tr) {
		t.Fatalf("forest=%v treasure on bf=%v; want the Forest", sc.tapped(f), sc.onBF(tr))
	}
	c = cost(0, kop.R)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if sc.onBF(tr) {
		t.Fatalf("treasure not sacrificed for {R}")
	}
}

// Any-colour production paying generic picks the smallest Produces vector:
// G before R before B before U before W (gorge pay/plan.go step order); a
// required pip forces its colour.
func TestAnyColourTieBreak(t *testing.T) {
	var pk planner
	var rem [kop.NumManaKinds]int
	var none [maxUnits]uint8
	if c := pk.pickColour(uint8(kop.ColAny), 0, 1, &rem, 0, &none, 0, 0); c != kop.G {
		t.Fatalf("generic from any colour = %d, want G", c)
	}
	rem[kop.W] = 1
	if c := pk.pickColour(uint8(kop.ColAny), 0, 1, &rem, 0, &none, 0, 0); c != kop.W {
		t.Fatalf("a {W} pip from any colour = %d, want W", c)
	}
	rem = [kop.NumManaKinds]int{}
	if c := pk.pickColour(uint8(kop.ColB|kop.ColR), 0, 1, &rem, 0, &none, 0, 0); c != kop.R {
		t.Fatalf("generic from a B/R gain land = %d, want R", c)
	}
}

// The lowered kicked cast (vanilla game 674 against gorge develop): gorge's
// witness for it is a composed-cost plan (potential_plan.go planComposedCost)
// planned with PlannedCast{}, so its hand demand counts the card being cast.
// Burst Lightning's {R} then makes key 6 keep a Mountain: of the two plans
// tied on keys 1-5 and 7 (three Islands + two Mountains, two Islands + three
// Mountains) it takes the first, although key 8 alone would take the
// second (witness 5,1,9,14,11 before 8,5,1,9,11 in entry order 8 5 1 9 14 11).
func TestAutopayKickedCountsCastCard(t *testing.T) {
	for _, kicked := range []bool{false, true} {
		sc := newScen(t, Config{})
		i1, i5, i8 := sc.obj(v.Island, 0), sc.obj(v.Island, 0), sc.obj(v.Island, 0)
		m9, m11, m14 := sc.obj(v.Mountain, 0), sc.obj(v.Mountain, 0), sc.obj(v.Mountain, 0)
		for _, o := range []Obj{i8, i5, i1, m9, m14, m11} {
			sc.enterAs(o, 0)
		}
		bl := sc.hand(v.BurstLightning, 0)
		if kicked {
			sc.s.beginCast(sc.k, bl, 1, 0)
		} else {
			// the same {4}{R} planned with the cast card excluded (a
			// planned cast's demand): key 8 decides
			c := cost(4, kop.R)
			sc.s.tapFor(sc.k, 0, &c, 0, bl)
		}
		keep := m14
		if !kicked {
			keep = i8
		}
		for _, o := range []Obj{i1, i5, i8, m9, m11, m14} {
			if sc.tapped(o) == (o == keep) {
				t.Fatalf("kicked=%v: obj %d tapped=%v; want every land but %d tapped", kicked, o, sc.tapped(o), keep)
			}
		}
	}
}

// TestAvailNineMasks: nine distinct production masks (five gain lands and
// four basics, pair 5901) are kept apart: one Island cannot pay
// Progenitus' {U}{U}. Before, the ninth mask (Plains) folded into the
// eighth (Island), and the folded W|U pair paid {U}{U}.
func TestAvailNineMasks(t *testing.T) {
	const W, U, B, R, G = 1, 2, 4, 8, 16
	var a avail
	for _, m := range []uint8{R | G, G | W, B | G, B | R, G, B, W | B, U, G, W} {
		a.addUnits(m, 1)
	}
	cost := kop.ManaCost{Pips: [6]uint8{2, 2, 2, 2, 2}}
	if a.affords(&cost, pipMaskOf(&cost), 0) {
		t.Fatal("ten lands with one blue source afford Progenitus")
	}
}

// A Treasure (a last-resort source) has colours 0 in gorge's rank census:
// keys 6 and 7 read the untapped NORMAL remainder in both phases (gorge
// pay/plan.go Alt.Colours). Paying {2}{W} with the first Treasure, a second
// untapped Treasure does not keep red in the remainder, so the plan taps
// two Swamps and keeps the Mountain (remainder {R,B}) rather than the
// lex-least Mountain + Swamp (remainder {B}). Pair 28254 and 11 more.
func TestAutopayTreasureNotInRemainder(t *testing.T) {
	sc := newScen(t, Config{})
	m := sc.perm(v.Mountain, 0)
	var sw [3]Obj
	for i := range sw {
		sw[i] = sc.perm(v.Swamp, 0)
	}
	var tr [2]Obj
	for i := range tr {
		tr[i] = sc.obj(v.TreasureToken, 0)
		sc.s.objs[tr[i]].of |= ofToken
		sc.enterAs(tr[i], 0)
	}
	c := cost(2, kop.W)
	sc.s.pay(sc.k, 0, &c, 0, NoObj)
	if sc.tapped(m) || !sc.tapped(sw[0]) || !sc.tapped(sw[1]) || sc.onBF(tr[0]) || !sc.onBF(tr[1]) {
		t.Fatalf("mountain=%v swamps=%v %v %v treasures on bf=%v %v; want two Swamps and the first Treasure",
			sc.tapped(m), sc.tapped(sw[0]), sc.tapped(sw[1]), sc.tapped(sw[2]), sc.onBF(tr[0]), sc.onBF(tr[1]))
	}
}
