package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// podSeats makes sc a four-seat game with every seat alive.
func podSeats(sc *scen) {
	sc.s.g.nseat = 4
	for p := uint8(0); p < 4; p++ {
		sc.s.pl[p].life = 20
	}
	if len(sc.s.cold.discHeld) == 0 {
		sc.s.cold.discHeld = make([]uint8, discHeldLen)
	}
}

// TestSpeedPoolHoldsFourSeats: genState.speed already holds four seats' speed
// (TestSpeedFourSeats), so a table carrying speed cards is no longer refused
// past two seats, and the latch above seat 3's bits is no seat's speed.
func TestSpeedPoolHoldsFourSeats(t *testing.T) {
	var s State
	s.speedAdd(3, 9)
	s.speedAdd(2, 3)
	s.g.speed |= speedLatch
	if s.Speed(3) != speedMaxV || s.Speed(2) != 3 || s.Speed(0) != 0 || s.g.speed&speedLatch == 0 {
		t.Fatalf("seat 3 raised to the cap changed a neighbour or the latch: %#x", s.g.speed)
	}
	set := cardsettest.Set(t, "speed")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	if sp, _ := prepSpeed(tbl); !sp {
		t.Fatal("the speed pool carries no speed card: the check is vacuous")
	}
	if TwoSeatOnly(tbl) {
		t.Fatalf("the speed pool is refused past two seats: %v", TwoSeatCauses(tbl))
	}
}

// skipUntapTable is the TestSkipUntap table: vanilla plus a sorcery whose
// program is OpSkipUntap, so the table carries the per-slot marks.
func skipUntapScen(t *testing.T) (*scen, CardID, kop.Instr) {
	t.Helper()
	tbl := *v.Table
	skip := kop.Instr{Op: kop.OpSkipUntap, Obj: kop.RefSelf}
	tbl.Exprs = make([]kop.Expr, 1)
	tbl.Cards = append(slices.Clone(v.Table.Cards), kop.CardDef{Name: "Test Freeze", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{skip}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	podSeats(sc)
	return sc, v.GrizzlyBears, skip
}

// TestSkipUntapPerSeat: each of four seats' OpSkipUntap marks is its own bit
// of the permanent's cold record, read at that seat's untap step only, and the
// step's end clears that seat's mark alone. The mark leaves with the slot's
// permanent (CR 400.7), and a table without the op keeps no region.
func TestSkipUntapPerSeat(t *testing.T) {
	sc, bear, skip := skipUntapScen(t)
	s, k := sc.s, sc.k
	if prepSkipUntapRegion := len(coldSkipUntap.of(s)); prepSkipUntapRegion != MaxPerm {
		t.Fatalf("the skip-untap table has %d marks, want %d", prepSkipUntapRegion, MaxPerm)
	}
	o := sc.perm(bear, 0)
	sl := sc.slot(o)
	for p := uint8(0); p < 4; p++ {
		s.res.e.ctrl, s.res.e.obj, s.res.e.srcVer = p, o, s.objs[o].ver
		s.exec(k, &skip)
	}
	if got := s.skipUntapMark(sl); got != 0xf {
		t.Fatalf("marks %#x, want 0xf (one bit per seat)", got)
	}
	for a := uint8(0); a < 4; a++ {
		s.active = a
		if !s.skipsUntap(k, sl) {
			t.Fatalf("seat %d's untap step does not skip it", a)
		}
		s.expireSkipUntap()
		if got, want := s.skipUntapMark(sl), uint8(0xf)&^(1<<(a+1)-1); got != want {
			t.Fatalf("after seat %d's step the marks are %#x, want %#x", a, got, want)
		}
		if s.skipsUntap(k, sl) && a == 3 {
			t.Fatal("seat 3's mark stays after its step")
		}
	}
	// a slot's next permanent starts unmarked
	s.res.e.ctrl = 2
	s.exec(k, &skip)
	s.coldClrSlot(sl)
	if s.skipUntapMark(sl) != 0 {
		t.Fatal("the mark outlived its permanent's slot")
	}
	plain := newScenTable(t, v.Table, Config{NoElide: true})
	if n := len(coldSkipUntap.of(plain.s)); n != 0 {
		t.Fatalf("a table without OpSkipUntap holds %d marks, want none", n)
	}
}

// TestBattleProtectorChosen: a battle entering in a pod asks its controller
// for the protector among the living opponents in seat order (CR 310.10);
// the answer is recorded, one living opponent is recorded unasked, and a
// protector leaving the game is replaced by the first living opponent.
func TestBattleProtectorChosen(t *testing.T) {
	sc, front, _ := battleScen(t, 3)
	podSeats(sc)
	s, k := sc.s, sc.k
	o := sc.perm(front, 1)
	sl := sc.slot(o)
	if s.g.nask != 1 || s.g.ask[0].kind != akProtector {
		t.Fatalf("queued asks %d (kind %d), want one protector ask", s.g.nask, s.g.ask[0].kind)
	}
	var d Decision
	d.Player = 1
	s.protectorOptions(&d)
	var seats []uint8
	for i := 0; i < int(d.N); i++ {
		seats = append(seats, uint8(d.Options[i].Mode()))
	}
	if d.Sub != ChGenPlayer || !slices.Equal(seats, []uint8{0, 2, 3}) {
		t.Fatalf("options %v (sub %d), want the opponents 0 2 3", seats, d.Sub)
	}
	s.answerQueuedAsk(k, MakeAction(AkChoose, 0, 0, 3))
	if pr, ok := s.protector(sl); !ok || pr != 3 {
		t.Fatalf("protector %d (%v), want 3", pr, ok)
	}
	// the protector leaves: the first living opponent takes over
	s.pl[3].lost = true
	s.rechooseProtectors(k, 3)
	if pr, _ := s.protector(sl); pr != 0 {
		t.Fatalf("after seat 3 left the protector is %d, want 0", pr)
	}
	// two live seats: no ask, the sole opponent protects
	sc2, front2, _ := battleScen(t, 3)
	podSeats(sc2)
	sc2.s.pl[2].lost, sc2.s.pl[3].lost = true, true
	o2 := sc2.perm(front2, 0)
	if sc2.s.g.nask != 0 {
		t.Fatalf("a single opponent was asked (%d asks)", sc2.s.g.nask)
	}
	if pr, ok := sc2.s.protector(sc2.slot(o2)); !ok || pr != 1 {
		t.Fatalf("sole-opponent protector %d (%v), want 1", pr, ok)
	}
}

// grantedGoadScen is a four-seat table with a creature whose Goad$ True static
// is switched on by an override grant (kop.Static.OvGate, the Animate
// staticAbilities$ route), and the grant's record on a permanent.
func grantedGoadScen(t *testing.T) (*scen, Obj, Slot) {
	t.Helper()
	tbl := *v.Table
	tbl.Exprs = make([]kop.Expr, 1)
	tbl.Filters = append(slices.Clone(v.Table.Filters), kop.Filter{Flags: kop.FSelf})
	tbl.Cards = slices.Clone(v.Table.Cards)
	bear := &tbl.Cards[v.GrizzlyBears]
	bear.Abilities = append(slices.Clone(bear.Abilities), kop.Ability{Kind: kop.AbStatic, Static: kop.Static{
		Kind: kop.StatGoad, OvGate: 1, Filter: kop.FilterID(len(v.Table.Filters))}})
	tbl.Overrides = append(slices.Clone(v.Table.Overrides), kop.Override{GrantStatics: 1})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	podSeats(sc)
	o := sc.perm(v.GrizzlyBears, 0)
	return sc, o, sc.slot(o)
}

// TestGrantedGoadSeat: a Goad$ True static an override grant switches on is
// goaded by the seat that made the grant, not by the holder's controller or
// every seat, and no grant means no goad (gorge staticGoaders: the Animate's
// controller). The table is no longer refused past two seats.
func TestGrantedGoadSeat(t *testing.T) {
	sc, o, sl := grantedGoadScen(t)
	s, k := sc.s, sc.k
	if TwoSeatOnly(k.t) {
		t.Fatalf("a granted static goad refuses a pod: %v", TwoSeatCauses(k.t))
	}
	if !k.gen.anyStatGoad {
		t.Fatal("the table carries no static goad")
	}
	for p := uint8(0); p < 4; p++ {
		if s.goadedBy(k, sl, p) {
			t.Fatalf("seat %d goads before any grant", p)
		}
	}
	s.res.e.ctrl, s.res.e.obj = 2, o // the Animate resolves under seat 2
	s.applyOverride(k, sl, len(k.t.Overrides)-1, s.perms[sl].card, false)
	for p := uint8(0); p < 4; p++ {
		if got := s.goadedBy(k, sl, p); got != (p == 2) {
			t.Fatalf("seat %d goads %v after seat 2's grant, want %v", p, got, p == 2)
		}
	}
}
