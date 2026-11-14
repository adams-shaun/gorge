package dirac

import (
	"slices"
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// denseCtr is the pre-M8 reference: every kind dense plus the seen mask.
type denseCtr struct {
	n    [kop.NumCounterKinds2]uint16
	seen uint16
}

func (d *denseCtr) held() int {
	k := 0
	for c := 1; c < int(kop.NumCounterKinds2); c++ {
		if d.n[c] != 0 || d.seen&(1<<c) != 0 {
			k++
		}
	}
	return k
}

// TestSparseCountersMatchDense drives the sparse slots with the overflow table
// and the dense reference through the same deterministic operation stream and
// checks every read, the seen mask, the stall point, and that equal counter
// states are equal bytes regardless of insertion order (ObjHash).
func TestSparseCountersMatchDense(t *testing.T) {
	x := uint32(1)
	rnd := func(n int) int { x = x*1664525 + 1013904223; return int(x>>8) % n }
	s, sq := &State{}, &State{}
	for run := 0; run < 2000; run++ {
		var p perm
		var d denseCtr
		s.g.nctrOver = 0
		for step := 0; step < 40; step++ {
			c := rnd(int(kop.NumCounterKinds2))
			if rnd(4) == 0 && c != 0 {
				c = 1 + rnd(4) // concentrate on a few kinds
			}
			switch rnd(4) {
			case 0, 1:
				n := 1 + rnd(3)
				fresh := c != 0 && d.n[c] == 0 && d.seen&(1<<c) == 0
				ok := p.addCtrN(s, c, n)
				if want := !(fresh && d.held() >= ctrSlots+maxCtrOver); ok != want {
					t.Fatalf("run %d: add kind %d ok=%v want %v", run, c, ok, want)
				}
				if ok {
					d.n[c] += uint16(n)
				}
			case 2:
				n := 1 + rnd(3)
				p.subCtrN(s, c, n)
				d.n[c] -= min(d.n[c], uint16(n))
			case 3:
				p.seeCtr(s)
				for k, v := range d.n {
					if v != 0 {
						d.seen |= 1 << k
					}
				}
			}
			for k := range d.n {
				if p.ctrN(s, k) != int(d.n[k]) {
					t.Fatalf("run %d step %d: kind %d = %d, dense %d", run, step, k, p.ctrN(s, k), d.n[k])
				}
			}
			if m := p.ctrSeenMask(s); m != d.seen {
				t.Fatalf("run %d step %d: seen %b, dense %b", run, step, m, d.seen)
			}
			// canonical: rebuilding the same state in descending kind order gives the same bytes
			var q perm
			sq.g.nctrOver = 0
			for k := int(kop.NumCounterKinds2) - 1; k >= 0; k-- {
				if d.seen&(1<<k) != 0 || d.n[k] != 0 {
					q.setCtrN(sq, k, 1)
				}
			}
			q.seeCtr(sq)
			for k := range d.n {
				if d.seen&(1<<k) != 0 || d.n[k] != 0 {
					q.setCtrN(sq, k, int(d.n[k]))
				}
			}
			// q marked every held kind seen; match only when p did too
			if p.ctrSeenMask(s) == q.ctrSeenMask(sq) && (p.ck != q.ck || p.cn != q.cn || p.p1 != q.p1 || p.cseen != q.cseen ||
				!slices.Equal(s.g.ctrOver[:s.g.nctrOver], sq.g.ctrOver[:sq.g.nctrOver])) {
				t.Fatalf("run %d step %d: same state, different bytes %v/%v vs %v/%v", run, step, p.ck, p.cn, q.ck, q.cn)
			}
		}
	}
}

// ctrOverScen is a permanent to pile counters on.
func ctrOverScen(t *testing.T) (*scen, Obj, map[string]CardID) {
	t.Helper()
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Zimone, Paradox Sculptor", Count: 2}, {Name: "Island", Count: 1}})
	return sc, sc.perm(id["Zimone, Paradox Sculptor"], 0), id
}

// ctrOverKinds are keyword-counter kinds, in the order the tests place them:
// not ascending, so later ones displace earlier ones from the slots.
var ctrOverKinds = []struct {
	kind kop.CounterKind
	kw   kop.KwMask
}{
	{kop.CtrTrample, kop.KwTrample},
	{kop.CtrFlying, kop.KwFlying},
	{kop.CtrReach, kop.KwReach},
	{kop.CtrFirstStrike, kop.KwFirstStrike},
	{kop.CtrDeathtouch, kop.KwDeathtouch},
	{kop.CtrLifelink, kop.KwLifelink},
}

func TestCtrOverKindsOnOnePermanent(t *testing.T) {
	for _, n := range []int{4, 5, 6} {
		sc, o, _ := ctrOverScen(t)
		sl := sc.slot(o)
		s, p := sc.s, &sc.s.perms[sl]
		for i, c := range ctrOverKinds[:n] {
			if got := s.placeCounters(sc.k, sl, int(c.kind), i+1, p.ctrl, ctrPutRaw); got != i+1 {
				t.Fatalf("%d kinds: placed %d of kind %d, want %d", n, got, c.kind, i+1)
			}
		}
		if s.Stalled() {
			t.Fatalf("%d kinds: stalled %s", n, StallName(s.StallReason()))
		}
		if int(s.g.nctrOver) != n-ctrSlots {
			t.Fatalf("%d kinds: %d table entries", n, s.g.nctrOver)
		}
		pr := &Projection{}
		s.Project(pr)
		po := &pr.Objs[o]
		_, kinds, _ := s.DebugCtrs(o)
		if len(kinds) != n {
			t.Fatalf("%d kinds: DebugCtrs lists %v", n, kinds)
		}
		kw, total := s.kw(sc.k, sl), 0
		for i, c := range ctrOverKinds[:n] {
			total += i + 1
			if got := p.ctrN(s, int(c.kind)); got != i+1 {
				t.Fatalf("%d kinds: kind %d holds %d, want %d", n, c.kind, got, i+1)
			}
			if kw&c.kw == 0 {
				t.Fatalf("%d kinds: kind %d grants no keyword", n, c.kind)
			}
			var proj uint16
			if c.kind < kop.NumCounterKinds2 {
				proj = po.Counters[c.kind]
			} else {
				proj = po.CountersX[c.kind-kop.CtrCharge]
			}
			if int(proj) != i+1 {
				t.Fatalf("%d kinds: kind %d projects %d, want %d", n, c.kind, proj, i+1)
			}
		}
		if got := permCtrN(s, p, int(kop.CtrAll)); got != total {
			t.Fatalf("%d kinds: %d counters in all, want %d", n, got, total)
		}
		h := s.ObjHash(o)
		p.setCtrN(s, int(ctrOverKinds[n-1].kind), n+1)
		if s.ObjHash(o) == h {
			t.Fatalf("%d kinds: ObjHash does not see a table entry", n)
		}
		p.setCtrN(s, int(ctrOverKinds[n-1].kind), n)
		// removing the kinds one at a time pulls the table back into the slots
		for i, c := range ctrOverKinds[:n] {
			p.setCtrN(s, int(c.kind), 0)
			for _, r := range ctrOverKinds[i+1 : n] {
				if p.ctrN(s, int(r.kind)) == 0 {
					t.Fatalf("%d kinds: removing kind %d lost kind %d", n, c.kind, r.kind)
				}
			}
		}
		if s.g.nctrOver != 0 || p.ck[0] != 0 {
			t.Fatalf("%d kinds: left %d entries, slots %v", n, s.g.nctrOver, p.ck)
		}
	}
}

// A kind removed to none stays listed, in the table as in a slot, and still
// takes its place when a slot frees.
func TestCtrOverSeen(t *testing.T) {
	s := &State{}
	var p perm
	for _, c := range []kop.CounterKind{kop.CtrM1M1, kop.CtrStun, kop.CtrShield, kop.CtrFlying, kop.CtrOther} {
		if !p.setCtrN(s, int(c), 1) {
			t.Fatal("no room")
		}
	}
	p.seeCtr(s)
	p.setCtrN(s, int(kop.CtrOther), 0)
	want := uint16(1<<kop.CtrM1M1 | 1<<kop.CtrStun | 1<<kop.CtrShield | 1<<kop.CtrFlying | 1<<kop.CtrOther)
	if m := p.ctrSeenMask(s); m != want || s.g.nctrOver != 2 {
		t.Fatalf("seen %b with %d entries, want %b with 2", m, s.g.nctrOver, want)
	}
}

func TestCtrOverReleasedOnLeave(t *testing.T) {
	sc, o, id := ctrOverScen(t)
	sl := sc.slot(o)
	s, p := sc.s, &sc.s.perms[sl]
	for _, c := range ctrOverKinds[:5] {
		s.placeCounters(sc.k, sl, int(c.kind), 2, p.ctrl, ctrPutRaw)
	}
	li := s.snapLKI(o)
	s.res.e.li = li
	if s.g.nctrOver != 4 {
		t.Fatalf("%d entries with one snapshot, want 2 live and 2 copies", s.g.nctrOver)
	}
	s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if len(s.ctrOverOf(o, 0)) != 0 {
		t.Fatal("the permanent's entries outlive it")
	}
	if n := s.lkiCounter(li, kop.CtrTrample); n != 2 {
		t.Fatalf("last known trample counters %d after leaving, want 2", n)
	}

	// a new object in the slot starts with none
	o2 := sc.perm(id["Zimone, Paradox Sculptor"], 0)
	if sc.slot(o2) != sl {
		t.Fatalf("new permanent took slot %d, want %d", sc.slot(o2), sl)
	}
	p2 := &s.perms[sl]
	for _, c := range ctrOverKinds {
		if p2.ctrN(s, int(c.kind)) != 0 {
			t.Fatalf("new object holds kind %d", c.kind)
		}
	}

	// the copies go with the snapshot: no holder is left, and the next snap sweeps
	s.res.e.li = 0
	s.snapLKI(o2)
	if s.g.nctrOver != 0 {
		t.Fatalf("%d entries after the snapshot's holder went", s.g.nctrOver)
	}
}

func TestCtrOverFullStalls(t *testing.T) {
	sc, o, _ := ctrOverScen(t)
	sl := sc.slot(o)
	s, p := sc.s, &sc.s.perms[sl]
	placed := 0
	for c := 1; placed < ctrSlots+maxCtrOver; c++ {
		if kop.IsCounterKind(c) && kop.CounterKind(c) != kop.CtrLevel {
			s.placeCounters(sc.k, sl, c, 1, p.ctrl, ctrPutRaw)
			placed++
		}
	}
	if s.Stalled() || int(s.g.nctrOver) != maxCtrOver {
		t.Fatalf("stalled %v with %d entries, want room for %d", s.Stalled(), s.g.nctrOver, maxCtrOver)
	}
	last := int(s.g.ctrOver[maxCtrOver-1].kind) + 1
	for !kop.IsCounterKind(last) {
		last++
	}
	if got := s.placeCounters(sc.k, sl, last, 1, p.ctrl, ctrPutRaw); got != 0 {
		t.Fatalf("placed %d past a full table", got)
	}
	if name := StallName(s.StallReason()); name != StallName(StallCounters) || s.StallReason() != StallCounters {
		t.Fatalf("stall %q, want %q", name, StallName(StallCounters))
	}
	if p.ctrN(s, last) != 0 {
		t.Fatal("a refused kind reads back")
	}
}

// TestPermSize pins perm's size. Under dirac_mp the object index and the
// codes are sixteen bits (obj, chosen, bord, atk: Obj, objWord, TCode), four
// bytes more with no pad byte (actUsed sits beside nblock to keep it so).
func TestPermSize(t *testing.T) {
	var p perm
	want := uintptr(56)
	if MaxSeats > 2 {
		want = 60
	}
	if got := unsafeSizeofPerm(&p); got != want {
		t.Fatalf("perm is %d bytes, want %d", got, want)
	}
}

func unsafeSizeofPerm(p *perm) uintptr { return unsafe.Sizeof(*p) }

// TestCtrSlotsStayPacked pins the invariant ctrLow's early exit rests on: the
// general counter slots never hold an empty slot before an occupied one, and
// the occupied ones stay in strictly ascending kind order, through the same
// operation stream as TestSparseCountersMatchDense.
func TestCtrSlotsStayPacked(t *testing.T) {
	x := uint32(1)
	rnd := func(n int) int { x = x*1664525 + 1013904223; return int(x>>8) % n }
	s := &State{}
	shifts := 0
	for run := 0; run < 2000; run++ {
		var p perm
		s.g.nctrOver = 0
		for step := 0; step < 40; step++ {
			c := rnd(int(kop.NumCounterKinds2))
			if rnd(4) == 0 && c != 0 {
				c = 1 + rnd(4) // concentrate on a few kinds
			}
			before := p.ck
			switch rnd(4) {
			case 0, 1:
				p.addCtrN(s, c, 1+rnd(3))
			case 2:
				p.subCtrN(s, c, 1+rnd(3))
			case 3:
				p.seeCtr(s)
			}
			for i := 1; i < ctrSlots; i++ {
				if p.ck[i] != 0 && p.ck[i-1] == 0 {
					t.Fatalf("run %d step %d: hole at slot %d: ck=%v", run, step, i-1, p.ck)
				}
				if p.ck[i] != 0 && p.ck[i] <= p.ck[i-1] {
					t.Fatalf("run %d step %d: slots out of order: ck=%v", run, step, p.ck)
				}
			}
			// a slot freed while a later one was occupied: that later kind moved down
			for i := 1; i < ctrSlots; i++ {
				if before[i-1] != 0 && before[i] != 0 && p.ck[i-1] == before[i] && !slices.Contains(p.ck[:ctrSlots], before[i-1]) {
					shifts++
					break
				}
			}
		}
	}
	if shifts == 0 {
		t.Fatal("precondition: the stream never freed a slot ahead of an occupied one, so ctrFree's shift went unexercised")
	}
	t.Logf("%d shifting frees", shifts)
}
