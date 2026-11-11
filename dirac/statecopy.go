package dirac

import (
	"slices"
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

// coldArrays are the arrays only some tables' cards reach. Each is a view of
// buf, one allocation per State, as long as the table's coldLayout says and
// empty when no card reaches it. An element type must hold no pointers: buf is
// a []uint64, which the garbage collector does not scan.
type coldArrays struct {
	layout   *coldLayout
	buf      []uint64
	ms       []slotMask   // slot-width mechanic masks (arenaAssign)
	mo       []objBits    // object-width mechanic masks
	exw      []exwRec     // exiled-with links
	atkGrp   []objBits    // attacker groups; stackEntry.grp - 1 indexes it
	exH      []exhRec     // until-end-of-turn can't-be-blocked-except records
	entered  []zoneEntry  // this turn's zone entries, in add order
	exMark   []exMarkRec  // this turn's exile-instead-of-dying effects
	parked   []parkedMove // moves waiting on a replacement order ask
	castRing []castPush   // this turn's spell pushes, castRingN per player
	discHeld []uint8      // the earlier players' discards while the next chooses
	copyHeld []copyHold   // the resolution waiting on its copy's election
	mk       []uint8      // per battlefield slot: multikicker payments of the permanent's spell (CR 702.43)
	mkObj    []uint8      // per object: the multikicker payments of the spell on the stack (CR 702.43)
}

// coldLayout is the lengths of a table's cold arrays.
type coldLayout struct {
	ms, mo, exw, atkGrp, exH, entered, exMark, parked, castRing, discHeld, copyHeld int
	xblk                                                                            int // xblkRec per battlefield slot, after the creature-counter record (combat_extrablock.go)
	may                                                                             int // mayPlay records past maxMay; the first thing in the buffer (cast_maycold.go)
	emb                                                                             int // embRec records for emblems past maxEmb, both players; right after the may records (static_emblemcold.go)
	ctrP                                                                            int // bytes of the per-player creature counter-plus-one record, right after the emblem records (repl_turnrepl.go)
	partner                                                                         int // cmdPartner records (0 or 1), right after the xblk records (zone_cmdpartner.go)
	mk                                                                              int // multikicker counts per slot (CR 702.43; cast.go)
	mkObj                                                                           int // multikicker counts per object while on the stack (cast.go)
	words                                                                           int
}

// prepCold sizes the arrays by the flags that cover their writers: exwPut's
// callers, grpNew's caller, ExtExHaste, noteEnter, noteCast and OpDiscard. A full exw, exH,
// atkGrp, entered or castRing stalls by name, and an empty exw or exH is simply full.
func prepCold(g *genPrep, l *coldLayout) {
	if g.anyExW || g.anyFaceExile {
		l.exw = maxExw
	}
	if g.anyTriggerAttackers || g.anyBatchPool {
		l.atkGrp = maxAtkGrp
	}
	if g.anyExHaste {
		l.exH = maxExH
	}
	if g.enteredNeed {
		l.entered = maxEntered
	}
	if g.ix[mechExileEx] != arenaNone {
		l.exMark = maxExMarks
	}
	if g.ix[mechExileEx] != arenaNone || g.anyRepl&rbExileOnLeave != 0 {
		l.parked = maxParked
	}
	if g.castRingOn || g.anyParadigm {
		l.castRing = MaxSeats * castRingN
	}
	if g.anyDiscard {
		l.discHeld = discHeldLen
	}
	if g.anyCopyElect {
		l.copyHeld = 1
	}
	if g.anyMayCold {
		l.may = maxMayCold
	}
	if g.anyEmbCold {
		l.emb = MaxSeats * maxEmbCold
	}
	if g.anyPlusCre {
		l.ctrP = 2 * MaxSeats
	}
	if g.anyExtraBlock {
		l.xblk = MaxPerm
	}
	if g.anyMultikick {
		l.mk = MaxPerm
		l.mkObj = MaxObj
	}
}

// place points c's arrays into b, or just counts them if b is nil, and returns
// the words they take.
func (l *coldLayout) place(c *coldArrays, b []uint64) int {
	at := 0
	_ = coldView[mayPlay](b, &at, l.may)        // first: mayCold reads it at the buffer start
	_ = coldView[embRec](b, &at, l.emb)         // next: embCold finds it after the may records
	_ = coldView[uint8](b, &at, l.ctrP)         // then plusCreCold, after the emblem records
	_ = coldView[xblkRec](b, &at, l.xblk)       // then xblkRecs, after the counter record
	_ = coldView[cmdPartner](b, &at, l.partner) // then partnerCold, after the xblk records
	c.ms = coldView[slotMask](b, &at, l.ms)
	c.mo = coldView[objBits](b, &at, l.mo)
	c.exw = coldView[exwRec](b, &at, l.exw)
	c.atkGrp = coldView[objBits](b, &at, l.atkGrp)
	c.exH = coldView[exhRec](b, &at, l.exH)
	c.entered = coldView[zoneEntry](b, &at, l.entered)
	c.exMark = coldView[exMarkRec](b, &at, l.exMark)
	c.parked = coldView[parkedMove](b, &at, l.parked)
	c.castRing = coldView[castPush](b, &at, l.castRing)
	c.discHeld = coldView[uint8](b, &at, l.discHeld)
	c.copyHeld = coldView[copyHold](b, &at, l.copyHeld)
	c.mk = coldView[uint8](b, &at, l.mk)
	c.mkObj = coldView[uint8](b, &at, l.mkObj)
	return (at + 7) / 8
}

// size fills l.words from the lengths.
func (l *coldLayout) size() { l.words = l.place(new(coldArrays), nil) }

func coldView[T any](b []uint64, at *int, n int) []T {
	var v T
	// at counts bytes: each view starts at its element's alignment, and only
	// the whole buffer rounds up to a word (place)
	p := (*at + int(unsafe.Alignof(v)) - 1) &^ (int(unsafe.Alignof(v)) - 1)
	*at = p + n*int(unsafe.Sizeof(v))
	if b == nil || n == 0 {
		return nil
	}
	return unsafe.Slice((*T)(unsafe.Add(unsafe.Pointer(&b[0]), p)), n)
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
}

// warmForkDepth is past the deepest nesting measured, 5 in random play.
const warmForkDepth = 8

// NewForkPool returns a pool that will not allocate for any measured nesting.
func NewForkPool() *ForkPool {
	p := &ForkPool{forks: make([]*State, warmForkDepth)}
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
	}
	f := p.forks[p.depth]
	p.depth++
	f.CopyFrom(s)
	return f
}

// unfork releases the most recent fork of s or of a State sharing its pool.
func (s *State) unfork() { s.pool.depth-- }
