package dirac

import (
	"slices"
	"strings"
	"sync"
	"unsafe"
)

// noCopy makes go vet's copylocks check reject any assignment of a State.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// CopyFrom makes dst a copy of src's game: every slot of perms ends up equal,
// though only those below either State's permHi need touching. dst keeps its
// own cold buffer and fork pool, and allocates only if its buffer is too small.
func (dst *State) CopyFrom(src *State) {
	stale := dst.permHi
	dst.stateCore = src.stateCore
	hi := src.permHi
	copy(dst.perms[:hi], src.perms[:hi])
	if stale > hi {
		clear(dst.perms[hi:stale])
	}
	if dst.cold.layout != src.cold.layout {
		dst.layOut(src.cold.layout)
	}
	copy(dst.cold.buf, src.cold.buf)
}

// SameGame reports whether s and t hold the same game.
func (s *State) SameGame(t *State) bool {
	return s.stateCore == t.stateCore && s.perms == t.perms && slices.Equal(s.cold.buf, t.cold.buf)
}

var noCold coldLayout

// layOut gives s the arrays of layout l, whose contents are then arbitrary.
// A nil l is a State never Init'd, which has none.
func (s *State) layOut(l *coldLayout) {
	s.cold.layout = l
	if l == nil {
		l = &noCold
	}
	if cap(s.cold.buf) < l.words {
		s.cold.buf = make([]uint64, l.words)
	}
	s.cold.buf = s.cold.buf[:l.words]
	l.place(&s.cold, s.cold.buf)
}

// resetCold gives s zeroed arrays of layout l, and makes sure the forks it
// will take need not allocate for them.
func (s *State) resetCold(l *coldLayout) {
	if s.cold.layout != l {
		s.layOut(l)
	}
	clear(s.cold.buf)
	if s.pool != nil {
		for _, f := range s.pool.forks {
			if cap(f.cold.buf) < l.words {
				f.cold.buf = make([]uint64, l.words)
			}
		}
	}
}

// ForkPool holds the States rules probes run on, one per nesting depth. A State
// and its forks share one pool, so a probe on a fork, or on the State itself
// while it holds a saved fork, takes the next depth down.
type ForkPool struct {
	forks []*State
	depth int
	// rootF[i] is the per-player turn facts (genState.turnF) of the State
	// that is no fork which forks[i] descends from, as they stood when its
	// outermost fork was taken: the search root's (static_turnbits.go
	// rootTurnF).
	rootF []uint8
}

// warmForkDepth is past the deepest nesting measured, 5 in random play.
const warmForkDepth = 8

// NewForkPool returns a pool that will not allocate for any measured nesting.
func NewForkPool() *ForkPool {
	p := &ForkPool{forks: make([]*State, warmForkDepth), rootF: make([]uint8, warmForkDepth)}
	for i := range p.forks {
		p.forks[i] = &State{pool: p}
	}
	return p
}

// UseForkPool makes s fork on p instead of allocating a pool of its own on its
// first fork. Every State on p must run on one goroutine: a fork is released in
// the call that took it, so forks on one goroutine nest whichever State took them.
func (s *State) UseForkPool(p *ForkPool) { s.pool = p }

// fork returns a copy of s to probe on. The caller pairs it with unfork, and
// the fork is not valid after that.
func (s *State) fork() *State {
	p := s.pool
	if p == nil {
		p = new(ForkPool)
		s.pool = p
	}
	if p.depth == len(p.forks) {
		f := new(State)
		f.pool = p
		p.forks = append(p.forks, f)
		p.rootF = append(p.rootF, 0)
	}
	root := s.rootTurnF()
	f := p.forks[p.depth]
	p.rootF[p.depth] = root
	p.depth++
	f.CopyFrom(s)
	return f
}

// unfork releases the most recent fork of s or of a State sharing its pool.
func (s *State) unfork() { s.pool.depth-- }

// The cold buffer. Arrays only some tables' cards reach live in one []uint64
// per State, laid out per table. Each array is a cold region, registered by
// the file of the feature that owns it, in a package-level var:
//
//	var coldAtkLast = newCold[uint8]("atkLast", func(g *genPrep) int { ... })
//
// and read through coldAtkLast.of(s). A new region touches no shared file:
// the registry orders the regions itself (coldRegistry.seal), so neither the
// order the files initialise in nor the order regions are added in reaches
// the layout.

// coldArrays is a State's cold buffer and the views of it that hot paths read
// without going through the layout: a region bound with view keeps one here
// (a slice header, so a new one grows State; TestStateLayoutPinned). An
// element type must hold no pointers: buf is a []uint64, which the garbage
// collector does not scan (TestColdArraysHoldNoPointers).
type coldArrays struct {
	layout *coldLayout
	buf    []uint64
	// slot-width mechanic masks (arena.go arenaAssign)
	ms []slotMask
	// object-width mechanic masks (arena.go)
	mo []objBits
	// exiled-with links (zone_exiledwith.go)
	exw []exwRec
	// attacker groups; stackEntry.grp - 1 indexes it (trig_attackers.go)
	atkGrp []objBits
	// until-end-of-turn can't-be-blocked-except records (static_cantblockexcept.go)
	exH []exhRec
	// this turn's zone entries, in add order (value_enteredledger.go)
	entered []zoneEntry
	// this turn's exile-instead-of-dying effects (repl_exilemark.go)
	exMark []exMarkRec
	// moves waiting on a replacement order ask (repl_exilemark.go)
	parked []parkedMove
	// this turn's spell pushes, castRingN per player (cast_castledger.go)
	castRing []castPush
	// the earlier players' discards while the next chooses (act_eachdiscard.go)
	discHeld []Obj
	// the resolution waiting on its copy's election (copy_gen.go)
	copyHeld []copyHold
	// per battlefield slot: multikicker payments of the permanent's spell (CR 702.43; kw_multikicker.go)
	mk []uint8
	// per object: the multikicker payments of the spell on the stack (CR 702.43; kw_multikicker.go)
	mkObj []uint8
	// per-slot timestamp of the mechHexUB grant (repl_turnrepl.go)
	hexUB []uint16
}

// maxCold is how many regions the registry holds; registering one more panics
// at start-up.
const maxCold = 64

// coldLayout is where a table's cold regions sit in the buffer, by registry
// position: n elements at byte offset off. set fills n, size the rest.
type coldLayout struct {
	n   [maxCold]int32
	off [maxCold]int32
	// the perSlot regions laid out, nslot of them (arenaClrSlot)
	slot  [maxCold]coldSlotSpan
	nslot int
	words int
}

// coldSlotSpan is a perSlot region: n records of sz bytes at off.
type coldSlotSpan struct{ off, sz, n int32 }

// coldDesc describes one region.
type coldDesc struct {
	key         string
	pos         int // its index in the sealed order (coldLayout.n)
	size, align int
	need        func(g *genPrep) int
	bind        func(c *coldArrays, p unsafe.Pointer, n int) // nil: no view in coldArrays
	slotReset   bool
	zero        any // a zero element, for TestColdArraysHoldNoPointers
	reg         *coldRegistry
}

// coldRegion is the handle a feature reads its region through.
type coldRegion[T any] struct{ *coldDesc }

// coldRegistry holds the regions. seal sorts them by alignment, widest
// first (no padding between them), then by key; keys are unique.
type coldRegistry struct {
	descs  []*coldDesc
	once   sync.Once
	sealed bool
}

// coldRegs is the package's registry.
var coldRegs coldRegistry

// newCold registers a region of T, need(g) elements long in a table whose
// generated prep is g (prepCold; a table with none has no regions; a nil need
// leaves the length to whoever lays the table out, as zone_cmdpartner.go's
// does). Call it only from a package-level var declaration.
func newCold[T any](key string, need func(g *genPrep) int) *coldRegion[T] {
	return coldRegister[T](&coldRegs, key, need)
}

func coldRegister[T any](r *coldRegistry, key string, need func(g *genPrep) int) *coldRegion[T] {
	if r.sealed {
		panic("dirac: cold region " + key + " registered after the layout was sealed")
	}
	var v T
	d := &coldDesc{key: key, size: int(unsafe.Sizeof(v)), align: int(unsafe.Alignof(v)), need: need, zero: v, reg: r}
	r.descs = append(r.descs, d)
	return &coldRegion[T]{d}
}

// view binds the region to a coldArrays field, which layOut points at it.
func (r *coldRegion[T]) view(f func(c *coldArrays) *[]T) *coldRegion[T] {
	r.bind = func(c *coldArrays, p unsafe.Pointer, n int) {
		if n == 0 {
			*f(c) = nil
			return
		}
		*f(c) = unsafe.Slice((*T)(p), n)
	}
	return r
}

// perSlot marks a region indexed by battlefield slot: arenaClrSlot zeroes a
// slot's record when the slot takes a new permanent.
func (r *coldRegion[T]) perSlot() *coldRegion[T] {
	r.slotReset = true
	return r
}

// seal fixes the order and each region's pos, once.
func (r *coldRegistry) seal() {
	r.once.Do(func() {
		if len(r.descs) > maxCold {
			panic("dirac: more cold regions than maxCold")
		}
		slices.SortFunc(r.descs, func(a, b *coldDesc) int {
			if a.align != b.align {
				return b.align - a.align
			}
			return strings.Compare(a.key, b.key)
		})
		for i, d := range r.descs {
			if i > 0 && r.descs[i-1].key == d.key {
				panic("dirac: cold region " + d.key + " registered twice")
			}
			d.pos = i
		}
		r.sealed = true
	})
}

// of is the region in s, nil when s's table has none of it.
func (r *coldRegion[T]) of(s *State) []T {
	l := s.cold.layout
	if l == nil {
		return nil
	}
	n := l.n[r.pos]
	if n == 0 {
		return nil
	}
	return unsafe.Slice((*T)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(s.cold.buf)), l.off[r.pos])), n)
}

// count is the region's length in l (0 for a nil l).
func (r *coldRegion[T]) count(l *coldLayout) int {
	if l == nil {
		return 0
	}
	return int(l.n[r.pos])
}

// set makes the region n elements long in l; l.size lays it out.
func (r *coldRegion[T]) set(l *coldLayout, n int) {
	r.reg.seal()
	l.n[r.pos] = int32(n)
}

// prepCold sizes every region of the package's registry for g.
func prepCold(g *genPrep, l *coldLayout) { coldRegs.prep(g, l) }

func (r *coldRegistry) prep(g *genPrep, l *coldLayout) {
	r.seal()
	for _, d := range r.descs {
		if d.need != nil {
			l.n[d.pos] = int32(d.need(g))
		}
	}
}

// size lays the regions out in the package's registry order.
func (l *coldLayout) size() { coldRegs.size(l) }

// size lays l's regions out in r's order. Each starts at its element's
// alignment, and only the whole buffer rounds up to a word; an empty region
// takes nothing, not even padding.
func (r *coldRegistry) size(l *coldLayout) {
	r.seal()
	at := 0
	l.nslot = 0
	for _, d := range r.descs {
		l.off[d.pos] = 0
		n := int(l.n[d.pos])
		if n == 0 {
			continue
		}
		at = (at + d.align - 1) &^ (d.align - 1)
		l.off[d.pos] = int32(at)
		if d.slotReset {
			l.slot[l.nslot] = coldSlotSpan{off: int32(at), sz: int32(d.size), n: int32(n)}
			l.nslot++
		}
		at += n * d.size
	}
	l.words = (at + 7) / 8
}

// place points c's views into b, which holds l.words words.
func (l *coldLayout) place(c *coldArrays, b []uint64) {
	coldRegs.seal()
	base := unsafe.Pointer(unsafe.SliceData(b))
	for _, d := range coldRegs.descs {
		if d.bind == nil {
			continue
		}
		n := int(l.n[d.pos])
		if n == 0 {
			d.bind(c, nil, 0)
			continue
		}
		d.bind(c, unsafe.Add(base, l.off[d.pos]), n)
	}
}

// coldClrSlot zeroes slot sl's record in every perSlot region of s.
func (s *State) coldClrSlot(sl Slot) {
	l := s.cold.layout
	if l == nil || l.nslot == 0 {
		return
	}
	base := unsafe.Pointer(unsafe.SliceData(s.cold.buf))
	for _, c := range l.slot[:l.nslot] {
		if int32(sl) < c.n {
			clear(unsafe.Slice((*byte)(unsafe.Add(base, int(c.off)+int(sl)*int(c.sz))), c.sz))
		}
	}
}
