package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The shared last-known-information pool (CR 608.2h, 603.2h,
// 400.7). A snapshot of a permanent as it last existed on the battlefield is
// taken when a trigger is queued (enqueueLKI), when a cost sacrifices the
// source (payCosts) and when a targeted spell starts resolving (the target's
// last known P/T). Each holder -- a stack entry, a pending trigger, the
// build (b.e), the resolution (res.e) and a delayed program (dpLKI) --
// keeps a 1-byte index into genState.lki: 0 means "no snapshot", i >= 1 is
// record lki[i-1].
//
// Reference counts are not maintained incrementally at every holder copy,
// shift and pop (a missed site would leak or free a live record). They are
// recomputed canonically by lkiSweep when a snapshot needs a record: a
// record's ref is the number of live holders (stack[:nstack], pend[:npend],
// b.e, res.e, dpLKI[:ndp]) naming it, and a record with ref 0 is free. So
// the pool's refcounts are a function of the holders, never of history.
// Identical snapshots of the same (obj, ver) share one record.
//
// Hash/projection stability: no hash (ViewHash, genViewHash, ObjHash) and
// no projection reads the pool or the holders' indices, so pool order and
// refcounts cannot make equal states hash differently. Readers only ever
// dereference an index, whose content is the snapshot itself.

// maxLKI is the LKI pool's records, at 56 B of State each on every pool:
// tliIdx's five bits hold up to 31. 31 since 2026-11-03, past MaxPending's old
// 24: every trigger of a creature entering snapshots it, so twenty Cat tokens
// from Ajani, Caller of the Pride beside Mentor of the Meek took twenty.
const maxLKI = 31

// lkiRec.bits (the byte that was faceDown): the snapshot's designations.
const (
	lkiFaceDown  uint8 = 1 << iota // CR 708.2: the snapshot was a face-down permanent
	lkiSuspected                   // CR 701.60b: the designation at the snapshot
	lkiTapped                      // CR 608.2h: "if it was tapped"
	lkiAttacking                   // CR 608.2h: "if it was attacking"
)

// lkiInfect (lkiRec.bits; bit 4 is zone_lkisource.go's lkiDepart): the
// snapshot had infect (infectAt; gorge lki.Infect, CR 113.7a), read by a
// departed DamageSource$ (act_damagefrom.go).
const lkiInfect uint8 = 1 << 5

// lkiRec is one snapshot: P/T, controller, every counter, and the derived
// (layered) types, colours and keywords (CR 608.2h in full).
type lkiRec struct {
	obj    Obj
	ver    uint8 // the object's ver when snapshot
	ref    uint8 // live holders at the last sweep (0 = free)
	ctrl   uint8
	power  int16
	tough  int16
	p1p1   uint16
	cn     [ctrSlots]uint16
	ck     [ctrSlots]uint8
	col    kop.ColorMask
	bits   uint8 // lkiFaceDown / lkiSuspected; the byte faceDown used
	chosen uint8 // perm.chosen (gorge SpecContext.SourceChosenType: a sacrificed source's choice; act_choosetype.go). The padding byte before types
	types  kop.TypeMask
	// subs is the derived subtype set at the snapshot, for the
	// Sacrificed$Valid Card/creature-type reads (anySacSub): the 192-bit
	// mask does not fit the 16-bit types snapshot, so it is a field of its
	// own. Zero unless the table lowers one of those reads.
	subs kop.SubMask
	kw   kop.KwMask
	mv   uint8 // mana value, the copied card's on a copy (CR 707.2, 608.2h)
}

var noLKI lkiRec // index 0: no snapshot (read-only)

// lk is the record index i names (the zero record for 0).
func (s *State) lk(i uint8) *lkiRec {
	if i == 0 {
		return &noLKI
	}
	return &s.g.lki[i-1]
}

// lkiCounter is record index i's last known count of counter kind c.
func (s *State) lkiCounter(i uint8, c kop.CounterKind) int {
	l := s.lk(i)
	if c == kop.CtrP1P1 {
		return int(l.p1p1)
	}
	for i := range l.ck {
		if l.ck[i] != 0 && kop.CounterKind(l.ck[i]) == c {
			return int(l.cn[i])
		}
	}
	if i != 0 && s.g.nctrOver != 0 {
		for _, e := range s.ctrOverOf(l.obj, i) {
			if e.kind == uint8(c) {
				return int(e.n)
			}
		}
	}
	return 0
}

// snapLKI snapshots object o's battlefield characteristics and returns its
// pool index (0 when o is not on the battlefield, or the pool is full: the
// game stalls, StallLKI).
//
//go:noinline
func (s *State) snapLKI(o Obj) uint8 {
	if o == NoObj || int(o) >= MaxObj || s.k().gen == nil {
		return 0
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return 0
	}
	k := s.k()
	sl := ob.slot
	p := &s.perms[sl]
	pw, t := s.pt(k, sl)
	r := lkiRec{obj: o, ver: ob.ver, ctrl: p.ctrl, power: int16(pw), tough: int16(t), p1p1: p.p1,
		col: s.colorsAt(k, sl), types: s.typesAt(k, sl), kw: s.kw(k, sl), chosen: p.chosen, mv: uint8(s.mvPerm(k, sl))}
	if s.faceDown(k, sl) {
		r.bits |= lkiFaceDown
	}
	if k.gen != nil && k.gen.anySacSub {
		r.subs = s.lkiSubs(k, sl) // Sacrificed$Valid Card/creature-type reads
	}
	if k.gen != nil && k.gen.anySuspect && s.suspected(k, sl) {
		r.bits |= lkiSuspected
	}
	if p.flags&pfTapped != 0 {
		r.bits |= lkiTapped
	}
	if p.flags&pfAttacking != 0 {
		r.bits |= lkiAttacking
	}
	if k.gen.anyInfect && s.infectAt(k, sl) {
		r.bits |= lkiInfect
	}
	for i := range p.ck {
		if p.cn[i] > 0 {
			r.ck[i], r.cn[i] = p.ck[i], p.cn[i]
		}
	}
	s.lkiSweep()
	free := -1
	for i := range s.g.lki {
		q := &s.g.lki[i]
		if q.ref == 0 {
			if free < 0 {
				free = i
			}
			continue
		}
		if r.obj == q.obj && r.ver == q.ver && r.same(q) && (s.g.nctrOver == 0 || s.ctrOverSnapSame(o, uint8(i+1))) {
			return uint8(i + 1)
		}
	}
	if free < 0 {
		s.capFull(StallLKI)
		return 0
	}
	r.ref = 1
	s.g.lki[free] = r
	if p.ck[ctrSlots-1] != 0 {
		s.ctrOverSnap(o, uint8(free+1))
	}
	return uint8(free + 1)
}

// lkiSubs is sl's derived subtype set as the snapshot takes it: the view's
// layer-aware type line, a printed changeling's every table creature type,
// and no subtype at all for a face-down permanent (CR 708.2). Only called
// when the table lowers a Sacrificed$Valid type read (genPrep.anySacSub).
func (s *State) lkiSubs(k *prep, sl Slot) kop.SubMask {
	if s.faceDown(k, sl) {
		return kop.SubMask{}
	}
	c := s.cardAt(sl)
	if _, subs, all, _, ok := s.FeatViewTypes(s.perms[sl].obj); ok {
		if all && k.gen != nil {
			subs = subs.Or(k.gen.creatSub)
		}
		return subs
	}
	subs := k.t.Cards[c].Subtypes
	all := k.t.Cards[c].AllCreatureTypes
	if k.gen != nil && k.gen.cpxSub && s.fxM.has(sl) {
		// A copy exception's changeling is a copiable all-creature-types
		// mark, not a printed CDA (Dominion Infiltration's token); the view
		// line is unchanged, so FeatViewTypes does not report it.
		subs = subs.Or(s.copyExcSubs(k, sl))
		all = all || s.copyExcAll(k, sl)
	}
	if all && k.gen != nil {
		subs = subs.Or(k.gen.creatSub)
	}
	return subs
}

// same: q holds r's snapshot (ref aside).
func (r *lkiRec) same(q *lkiRec) bool {
	return r.ctrl == q.ctrl && r.power == q.power && r.tough == q.tough && r.p1p1 == q.p1p1 &&
		r.cn == q.cn && r.ck == q.ck && r.col == q.col && r.types == q.types && r.subs == q.subs && r.kw == q.kw &&
		r.bits == q.bits && r.chosen == q.chosen
}

// lkiSweep recomputes every record's reference count from the live holders.
func (s *State) lkiSweep() {
	g := &s.g
	for i := range g.lki {
		g.lki[i].ref = 0
	}
	for i := 0; i < int(s.sp); i++ {
		s.lkiHold(s.stack[i].li)
	}
	for i := 0; i < int(s.npend); i++ {
		s.lkiHold(s.pend[i].li)
	}
	for i := 0; i < int(g.ndp); i++ {
		s.lkiHold(g.dpLKI[i])
	}
	if s.buildLive() {
		s.lkiHold(s.bld().e.li) // a finished build only leaves stale bytes (finishBuild copied the entry, li included, to the stack)
	}
	s.lkiHold(s.res.e.li)
	if s.res.active {
		s.lkiHold(s.g.tli & tliIdx)
	}
	s.srcDepartHold() // the departure snapshots of a live ability's source (zone_lkisource.go)
	if g.nctrOver != 0 {
		s.ctrOverSweep()
	}
}

func (s *State) lkiHold(i uint8) {
	if i != 0 && s.g.lki[i-1].ref < 255 {
		s.g.lki[i-1].ref++
	}
}

// objVer is o's ver (0 for NoObj).
func (s *State) objVer(o Obj) uint8 {
	if o >= MaxObj {
		return 0
	}
	return s.objs[o].ver
}

const (
	tliOn     = 0x80 // res.tli: matchTargetLKI is reading the snapshot
	tliRemLKI = 0x40 // the snapshot is a RememberLKI$ move's capture: a remembered read takes its controller (zone_rememberparams.go)
	tliIdx    = 0x1F // the record index (1..maxLKI)
)

// snapResTarget: a resolving effect is about to move o off the battlefield;
// when o is one of the resolution's targets its last known information is
// held in g.tli for the rest of the resolution (CR 608.2h: Driftgloom
// Coyote's "if that creature had power 2 or less").
//
//go:noinline
func (s *State) snapResTarget(o Obj) {
	e := &s.res.e
	for i := 0; i < len(e.tgt); i++ {
		if e.tgt[i] == TCode(o) {
			if li := s.snapLKI(o); li != 0 {
				s.g.tli = li
			}
			return
		}
	}
}

// matchTargetLKI matches a target the resolution moved off the battlefield
// as gorge does (conditionDefinedMet, targetCountersLKI): the card in its
// new zone; its power, toughness, tapped and attacking status are the last
// known ones (lkiPT, lkiStatusOK), every other characteristic is read live.
// ok is false when no snapshot covers o.
//
//go:noinline
func (s *State) matchTargetLKI(k *prep, f kop.FilterID, o Obj, ctrl uint8) (m, ok bool) {
	l := s.lk(s.g.tli & tliIdx)
	ob := &s.objs[o]
	if l.obj != o || ob.ver != l.ver+1 {
		return false, false
	}
	s.g.tli |= tliOn
	m = s.matchCardGen(k, f, ob.card, ob.owner(), ctrl, o)
	s.g.tli &^= tliOn
	return m, true
}

// lkiStatusOK reads a filter's tapped and attacking flags from g.tli's
// snapshot. Blocking is not captured: a moved target reads as not blocking.
//
//go:noinline
func (s *State) lkiStatusOK(fl *kop.Filter) bool {
	bits := s.lk(s.g.tli & tliIdx).bits
	tapped, attacking := bits&lkiTapped != 0, bits&lkiAttacking != 0
	return (fl.Flags&kop.FTapped == 0 || tapped) && (fl.Flags&kop.FUntapped == 0 || !tapped) &&
		(fl.Flags&kop.FAttacking == 0 || attacking) && (fl.Flags&kop.FNotAttacking == 0 || !attacking) &&
		fl.Flags&kop.FBlocking == 0 && (fl.Flags&kop.FAttackingOrBlocking == 0 || attacking)
}

// lkiPT is the last known (layer-derived) P/T of g.tli's snapshot:
// gorge (CR 608.2h) reads a departed target's battlefield
// power/toughness, statics and counters alike, not the printed P/T plus
// counters.
//
//go:noinline
func (s *State) lkiPT() (int, int) {
	l := s.lk(s.g.tli & tliIdx)
	return int(l.power), int(l.tough)
}
