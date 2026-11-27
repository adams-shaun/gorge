package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The per-decision memo of scriptFilt (docs/design/dirac-static-source-cache.md,
// follow-up 1). One priority walk asks scriptFilt about 6 times per player; the
// answer is a slot mask of p's activatable filter sources, priced against the
// board, p's floating pool and the potential bound. Nothing it reads changes
// inside one walk on the State that opened it, so the mask is kept per seat.
//
// Like the manaPCGen memo it lives in the ForkPool (never State), holds no
// pointers, and answers only when all hold:
//   - the memo is open (sfmOpen at stepRaw's priority arm; sfmClose and the
//     transition boundaries close it);
//   - the asker is the State that opened it, no fork of it is out, no filter
//     search or census window runs, and no in-place probe (pcMemoHold) holds it;
//   - the entry was written in this opening (gen), for the same seat, under the
//     same inputs filtBoundPays' own memo keys on: the object-write count, the
//     availFor bits, the stack depth, p's floating pool, the priority probe,
//     the self target, the play-anything flag, the line of play and life.
//
// Anything else recomputes. DebugSFMemoShadow recomputes each hit and panics
// when the answers differ.

type sfKey struct {
	touch   uint32
	avx     uint16
	sp, p   uint8
	mayAny  bool
	pool    [kop.NumManaKinds]uint8
	tgtSelf Obj
	pr      probe
	line    uint32
	life    int16
}

type sfMemoEnt struct {
	gen uint16
	key sfKey
	m   slotMask
}

type sfMemo struct {
	owner uintptr // the State that opened it (identity, never dereferenced); 0: shut
	gen   uint16
	ent   [MaxSeats]sfMemoEnt
	hits  uint32
}

// DebugSFMemoOff turns the memo off (equivalence test and benchmarks).
var DebugSFMemoOff bool

// DebugSFMemoShadow recomputes every hit and panics when it differs.
var DebugSFMemoShadow bool

// debugSFMemoNoInval drops the invalidation (generation bump and the write
// count in the key): the mutation the shadow check must catch.
var debugSFMemoNoInval bool

func (s *State) sfmOpen(k *prep) {
	if DebugSFMemoOff || k.gen == nil || !k.gen.anyFilt {
		return
	}
	p := s.pool
	if p == nil {
		p = new(ForkPool)
		s.pool = p
	}
	m := &p.sfm
	m.next()
	m.owner = s.addr()
}

func (m *sfMemo) next() {
	if debugSFMemoNoInval {
		return
	}
	m.gen++
	if m.gen == 0 {
		clear(m.ent[:])
		m.gen = 1
	}
}

func (s *State) sfmClose() {
	if p := s.pool; p != nil {
		p.sfm.owner = 0
	}
}

// sfmDrop is CopyFrom's: the object-write count is copied with the game.
func (s *State) sfmDrop() {
	if p := s.pool; p != nil && p.sfm.owner == s.addr() {
		p.sfm.next()
	}
}

func (s *State) sfmLive() *sfMemo {
	p := s.pool
	if p == nil || p.sfm.owner != s.addr() || p.depth|p.inFilt != 0 || p.census || p.pcm.hold != 0 {
		return nil
	}
	return &p.sfm
}

// scriptFilt: p's activatable filter sources (untapped), a slot mask, through
// the per-decision memo (scriptFiltCalc is the computation).
func (s *State) scriptFilt(k *prep, p uint8) slotMask {
	if !k.gen.anyFilt {
		return slotMask{}
	}
	m := s.sfmLive()
	if m == nil {
		return s.scriptFiltCalc(k, p)
	}
	key := sfKey{touch: s.touchN, avx: s.g.availX, sp: s.sp, p: p, mayAny: s.g.mayPlayAny,
		pool: s.pl[SeatIx(p)].pool, tgtSelf: s.g.tgtSelf, pr: s.g.probe,
		line: uint32(s.line ^ s.line>>32), life: s.pl[SeatIx(p)].life}
	if debugSFMemoNoInval {
		key.touch = 0
	}
	e := &m.ent[SeatIx(p)]
	if e.gen == m.gen && e.key == key {
		m.hits++
		if DebugSFMemoShadow {
			if calc := s.scriptFiltCalc(k, p); calc != e.m {
				panic("dirac: scriptFilt memo answered a stale mask")
			}
		}
		return e.m
	}
	r := s.scriptFiltCalc(k, p)
	*e = sfMemoEnt{gen: m.gen, key: key, m: r}
	return r
}
