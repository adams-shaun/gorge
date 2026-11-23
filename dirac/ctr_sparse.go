package dirac

import (
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cold paths of the sparse counter slots (perm.ck/cn/cseen) and of
// the overflow table behind them.
//
// A permanent's general slots hold its ctrSlots lowest kinds. Its higher kinds
// are entries of genState.ctrOver, which only a permanent with every slot taken
// has. So equal counter states stay equal bytes, whatever order the kinds came
// in, and a read that hits a slot never looks at the table.

// maxCtrOver is the overflow table's entries: every counter kind held past
// the slots by any permanent, plus the LKI snapshots' copies of them. The
// keyword-counter pool of TestCtrcellLockstep peaks at 8 in 600 games. 32
// since 2026-11-19 (16 before, about 100 bytes of State; the 16 more cost 128):
// a four-seat MBC pod filled 16, and the LKI pool's records, which copy a
// snapshot's overflow kinds here, grew from 31 to 63.
const maxCtrOver = 32

// ctrPadN pads genState so the 16-bit counter kinds (perm.ck, lkiRec.ck,
// ctrOverEntry.kind, genState's lastCK and enterCtr.kind) grow the State ahead
// of perms by a whole number of cache lines and perms keeps its phase
// (statelayout_test.go): with a two-byte Obj they grew it by 256 bytes
// (lkiRec 56 -> 64 bytes), already whole lines, so the pad is 0; a one-byte
// Obj grew it by 48, which 16 carried to 64.
const ctrPadN = 16 * (2 - int(unsafe.Sizeof(Obj(0))))

// ctrOverEntry is one counter kind in the overflow table. lki 0 is a kind obj
// holds now; i+1 is LKI record i's copy of a kind obj held when it was taken.
type ctrOverEntry struct {
	n    uint16
	kind uint16 // a kop.CounterKind, as wide as perm.ck
	obj  Obj
	lki  uint8
	seen bool   // perm.cseen's bit for this kind
	ts   uint16 // a keyword counter's CR 613.7 stamp (perm.cs)
}

// key orders the table by lki and object, then kind. The lki and object half is
// the one the 8-bit kinds had (lki<<16 | obj<<8), shifted past the 16-bit kind,
// so the order of every table holding kinds below 256 is unchanged.
func (e *ctrOverEntry) key() uint64 {
	return uint64(uint32(e.lki)<<16|uint32(e.obj)<<8)<<8 | uint64(e.kind)
}

// DebugCtrOverProjOff is a test-only mutation: Project ignores the overflow
// table, as a reader that only looks at a permanent's slots would.
var DebugCtrOverProjOff bool

// ctrOverOf is the entries of object o's kinds (lki 0) or of LKI record lki-1's
// copies, in ascending kind order.
func (s *State) ctrOverOf(o Obj, lki uint8) []ctrOverEntry {
	t := s.g.ctrOver[:s.g.nctrOver]
	lo := 0
	for lo < len(t) && (t[lo].obj != o || t[lo].lki != lki) {
		lo++
	}
	hi := lo
	for hi < len(t) && t[hi].obj == o && t[hi].lki == lki {
		hi++
	}
	return t[lo:hi]
}

// ctrOverFind is the index of object o's entry of kind c, or -1.
func (s *State) ctrOverFind(o Obj, c int) int {
	for i := 0; i < int(s.g.nctrOver); i++ {
		if e := &s.g.ctrOver[i]; e.lki == 0 && e.obj == o && int(e.kind) == c {
			return i
		}
	}
	return -1
}

// ctrOverPut adds an entry, keeping the table sorted. It reports false when the
// table is full.
func (s *State) ctrOverPut(e ctrOverEntry) bool {
	g := &s.g
	if int(g.nctrOver) == maxCtrOver {
		return false
	}
	at := int(g.nctrOver)
	for at > 0 && g.ctrOver[at-1].key() > e.key() {
		g.ctrOver[at] = g.ctrOver[at-1]
		at--
	}
	g.ctrOver[at] = e
	g.nctrOver++
	return true
}

// ctrOverDel removes entry i.
func (s *State) ctrOverDel(i int) {
	g := &s.g
	copy(g.ctrOver[i:g.nctrOver-1], g.ctrOver[i+1:g.nctrOver])
	g.nctrOver--
	g.ctrOver[g.nctrOver] = ctrOverEntry{}
}

// ctrOverN is object o's count of kind c in the table.
//
//go:noinline
func (s *State) ctrOverN(o Obj, c int) int {
	if i := s.ctrOverFind(o, c); i >= 0 {
		return int(s.g.ctrOver[i].n)
	}
	return 0
}

// ctrOverSet sets object o's count of kind c if the table holds that kind,
// dropping an entry at 0 that was never seen. It reports whether it did.
//
//go:noinline
func (s *State) ctrOverSet(o Obj, c int, n uint16) bool {
	i := s.ctrOverFind(o, c)
	if i < 0 {
		return false
	}
	if n > s.g.ctrOver[i].n && ctrKindKw(s.g.ctrOver[i].kind) != 0 {
		s.g.ctrOver[i].ts = s.tick()
	}
	s.g.ctrOver[i].n = n
	if n == 0 && !s.g.ctrOver[i].seen {
		s.ctrOverDel(i)
	}
	return true
}

// ctrOverSee marks seen the kinds object o holds in the table (perm.seeCtr).
//
//go:noinline
func (s *State) ctrOverSee(o Obj) {
	t := s.ctrOverOf(o, 0)
	for i := range t {
		if t[i].n != 0 && t[i].kind < uint16(kop.NumCounterKinds2) {
			t[i].seen = true
		}
	}
}

// ctrOverSum is the counters in the entries of ctrOverOf(o, lki). A Class's
// level is not a counter.
//
//go:noinline
func (s *State) ctrOverSum(o Obj, lki uint8) int {
	n := 0
	for _, e := range s.ctrOverOf(o, lki) {
		if kop.CounterKind(e.kind) != kop.CtrLevel {
			n += int(e.n)
		}
	}
	return n
}

// ctrOverHash mixes object o's entries into its content hash.
//
//go:noinline
func (s *State) ctrOverHash(h uint64, o Obj) uint64 {
	for _, e := range s.ctrOverOf(o, 0) {
		v := uint64(e.kind&0xff)<<16 | uint64(e.n)
		if e.seen {
			v |= 1 << 24
		}
		// a kind's high byte folds in above the 8-bit kinds' layout, so a
		// kind below 256 hashes as it did
		h = hmix(h, (0x6374726f<<32|v)^uint64(e.kind>>8)<<40)
		if e.ts != 0 {
			h = hmix(h, 0x63747374<<32|uint64(e.ts))
		}
	}
	return h
}

// ctrOverRelease drops the entries of a permanent leaving the battlefield. Its
// LKI records keep their copies.
//
//go:noinline
func (s *State) ctrOverRelease(o Obj) {
	for i := int(s.g.nctrOver) - 1; i >= 0; i-- {
		if e := &s.g.ctrOver[i]; e.lki == 0 && e.obj == o {
			s.ctrOverDel(i)
		}
	}
}

// ctrOverSnap copies object o's held kinds to LKI record lki-1. A full table
// stalls: the record would misstate the counters.
//
//go:noinline
func (s *State) ctrOverSnap(o Obj, lki uint8) {
	// lki 0 sorts first, so the additions don't move these entries
	for _, e := range s.ctrOverOf(o, 0) {
		if e.n == 0 {
			continue
		}
		if !s.ctrOverPut(ctrOverEntry{n: e.n, obj: o, kind: e.kind, lki: lki}) {
			s.ctrStall()
			return
		}
	}
}

// ctrOverSnapSame reports whether LKI record lki-1's copies are object o's held
// kinds now.
//
//go:noinline
func (s *State) ctrOverSnapSame(o Obj, lki uint8) bool {
	snap := s.ctrOverOf(o, lki)
	j := 0
	for _, e := range s.ctrOverOf(o, 0) {
		if e.n == 0 {
			continue
		}
		if j == len(snap) || snap[j].kind != e.kind || snap[j].n != e.n {
			return false
		}
		j++
	}
	return j == len(snap)
}

// ctrOverSweep drops the copies of LKI records no holder keeps (lkiSweep).
//
//go:noinline
func (s *State) ctrOverSweep() {
	for i := int(s.g.nctrOver) - 1; i >= 0; i-- {
		if e := &s.g.ctrOver[i]; e.lki != 0 && s.g.lki[e.lki-1].ref == 0 {
			s.ctrOverDel(i)
		}
	}
}

// ctrInsert puts n counters of kind c (not +1/+1, n > 0) on p, keeping the
// occupied slots packed in ascending kind order with their seen bits. With
// every slot taken, the highest of c and the slots' kinds goes to the overflow
// table. It reports false, changing nothing, when the table is full.
//
//go:noinline
func (p *perm) ctrInsert(s *State, c int, n uint16) bool {
	used := 0
	for used < ctrSlots && p.ck[used] != 0 {
		used++
	}
	if used == ctrSlots {
		top := ctrSlots - 1
		if c > int(p.ck[top]) {
			return s.ctrOverPut(ctrOverEntry{n: n, obj: p.obj, kind: uint16(c), ts: s.kwStamp(c)})
		}
		if !s.ctrOverPut(ctrOverEntry{n: p.cn[top], obj: p.obj, kind: p.ck[top], seen: p.cseen&(1<<top) != 0, ts: p.cs[top]}) {
			return false
		}
		used = top
	}
	at := used
	for at > 0 && int(p.ck[at-1]) > c {
		at--
	}
	for i := used; i > at; i-- {
		p.ck[i], p.cn[i], p.cs[i] = p.ck[i-1], p.cn[i-1], p.cs[i-1]
		p.cseen = p.cseen&^(1<<i) | (p.cseen<<1)&(1<<i)
	}
	p.ck[at], p.cn[at], p.cs[at] = uint16(c), n, s.kwStamp(c)
	p.cseen &^= 1 << at
	return true
}

// ctrFree empties general slot i (count 0, never seen), shifting the later
// slots down so the occupied ones stay packed. The lowest overflow kind, if p
// has one, takes the last slot.
//
//go:noinline
func (p *perm) ctrFree(s *State, i int) {
	full := p.ck[ctrSlots-1] != 0
	for ; i < ctrSlots-1; i++ {
		p.ck[i], p.cn[i], p.cs[i] = p.ck[i+1], p.cn[i+1], p.cs[i+1]
		p.cseen = p.cseen&^(1<<i) | (p.cseen>>1)&(1<<i)
	}
	p.ck[i], p.cn[i], p.cs[i] = 0, 0, 0
	p.cseen &^= 1 << i
	if !full {
		return
	}
	if t := s.ctrOverOf(p.obj, 0); len(t) != 0 {
		e := t[0]
		p.ck[i], p.cn[i], p.cs[i] = e.kind, e.n, e.ts
		if e.seen {
			p.cseen |= 1 << i
		}
		s.ctrOverDel(s.ctrOverFind(p.obj, int(e.kind)))
	}
}

// ctrHeld is room for every counter kind one permanent can hold at once: its
// +1/+1 count, its general slots and the whole overflow table.
type ctrHeld [1 + ctrSlots + maxCtrOver]uint16

// heldKinds writes to out the counter kinds p holds with a positive count, in
// ascending kind order, and returns how many: exactly the kinds c a walk of
// the whole kind space finds with ctrN(s, c) > 0, in that walk's order, at a
// cost that does not grow with kop.NumCounterKinds3. It includes CtrLevel,
// which is no counter: a walk filters it with kop.IsCounterKind as before.
// The list is a snapshot: a walk that puts counters while it iterates puts
// them of the kind it is on, so the kinds held don't change under it.
func (p *perm) heldKinds(s *State, out *ctrHeld) int {
	n := 0
	if p.p1 != 0 {
		out[0] = uint16(kop.CtrP1P1)
		n = 1
	}
	// the occupied slots are packed in ascending kind order (ctrInsert,
	// ctrFree); a seen slot at count 0 holds nothing
	for i := range p.ck {
		if p.ck[i] == 0 {
			break
		}
		if p.cn[i] != 0 {
			out[n] = p.ck[i]
			n++
		}
	}
	if p.ck[ctrSlots-1] == 0 {
		return n // as ctrN: only a permanent with every slot taken has entries
	}
	// the entries are in ascending kind order and above the slots' kinds;
	// the insertion keeps the list ascending without relying on that
	for _, e := range s.ctrOverOf(p.obj, 0) {
		if e.n == 0 {
			continue
		}
		at := n
		for at > 0 && out[at-1] > e.kind {
			out[at] = out[at-1]
			at--
		}
		out[at] = e.kind
		n++
	}
	return n
}

// ctrSeenMask is p's seen counter kinds as a mask over FeatCtrKinds.
func (p *perm) ctrSeenMask(s *State) uint16 {
	var m uint16
	for i := range p.ck {
		if p.cseen&(1<<i) != 0 {
			m |= 1 << p.ck[i]
		}
	}
	if p.ck[ctrSlots-1] != 0 {
		for _, e := range s.ctrOverOf(p.obj, 0) {
			if e.seen {
				m |= 1 << e.kind
			}
		}
	}
	if p.cseen&cseenP1 != 0 {
		m |= 1 << kop.CtrP1P1
	}
	if p.cseen&cseenDT != 0 {
		m |= 1 << FeatCtrDeathtouched
	}
	return m
}

// ctrStall stalls the game on a counter kind the overflow table has no room for
//
//go:noinline
func (s *State) ctrStall() { s.capFull(StallCounters) }

// kwStamp is the CR 613.7 timestamp a new counter of kind c takes: a fresh
// tick for a keyword counter (CR 122.1b; gorge stampKeywordCounter ticks
// Game.Clock for it and for no other kind), 0 for the rest.
func (s *State) kwStamp(c int) uint16 {
	if ctrKindKw(uint16(c)) == 0 {
		return 0
	}
	return s.tick()
}
