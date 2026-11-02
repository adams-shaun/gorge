package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// blgScen is the hybridn pool table's scenario with p0's board, each name
// once (static_landtype.go).
func blgScen(t *testing.T, board ...string) (*scen, []Obj) {
	t.Helper()
	set := cardsettest.Set(t, "hybridn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = CardID(c)
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{})
	if !sc.k.gen.blgOn {
		t.Fatal("blgOn unset: Leyline's AddSub carries no basic land type")
	}
	var objs []Obj
	for _, n := range board {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the hybridn table", n)
		}
		objs = append(objs, sc.perm(c, 0))
	}
	return sc, objs
}

// A Plains under Leyline of the Guildpact has every basic land type and so
// the five intrinsic abilities (CR 305.6): it affords each of {U} {B} {R}
// {G} alone, one tap (not two of them), and pays a {B} cost; a Plains
// without Leyline makes only {W}.
func TestBasicLandGrantMana(t *testing.T) {
	cols := []int{kop.U, kop.B, kop.R, kop.G}
	sc, objs := blgScen(t, "Leyline of the Guildpact", "Plains")
	pl := objs[1]
	var av avail
	sc.s.availFor(sc.k, 0, &av)
	for _, c := range cols {
		cs := cost(0, c)
		if !av.affords(&cs, pipMaskOf(&cs), 0) {
			t.Errorf("a Plains under Leyline does not afford pip %d", c)
		}
	}
	two := cost(0, kop.U, kop.B)
	if av.affords(&two, pipMaskOf(&two), 0) {
		t.Error("one Plains affords {U}{B}: a source is one tap")
	}
	if sc.s.StallReason() != StallNone {
		t.Fatalf("stalled %s", StallName(sc.s.StallReason()))
	}
	b := cost(0, kop.B)
	sc.s.pay(sc.k, 0, &b, 0, NoObj)
	if !sc.tapped(pl) || sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("Plains tapped %v pool %v; want tapped, nothing floating", sc.tapped(pl), sc.s.pl[0].pool)
	}

	sc, _ = blgScen(t, "Plains")
	sc.s.availFor(sc.k, 0, &av)
	for _, c := range cols {
		cs := cost(0, c)
		if av.affords(&cs, pipMaskOf(&cs), 0) {
			t.Errorf("a Plains alone affords pip %d", c)
		}
	}
	w := cost(0, kop.W)
	if !av.affords(&w, pipMaskOf(&w), 0) {
		t.Error("a Plains alone does not afford {W}")
	}
}

// The payment window lists what the cost owes first, a granted ability included
// (gorge owedAbilitiesFirst): a Forest under Leyline makes the owed {R}, and the
// Mountain beside it stays untapped.
func TestBasicLandGrantWindow(t *testing.T) {
	sc, objs := blgScen(t, "Leyline of the Guildpact", "Forest", "Mountain")
	c := cost(0, kop.R)
	if ok, _ := sc.s.windowTap(sc.k, 0, &c, false); !ok {
		t.Fatal("window did not pay")
	}
	if !sc.tapped(objs[1]) || sc.tapped(objs[2]) || sc.s.pl[0].pool[kop.R] != 1 {
		t.Fatalf("tapped %v %v pool %v; want the Forest alone, {R} floating", sc.tapped(objs[1]), sc.tapped(objs[2]), sc.s.pl[0].pool)
	}
}
