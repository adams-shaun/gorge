package dirac

import (
	"encoding/binary"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// marks.go is the change tracking an incremental reader of a State (the
// network-input encoder in dirac/feat and the evaluation cache in
// the network evaluator) keys on.
//
// Every write to an object (objs[o]) or to the battlefield slot it occupies
// (perms[slot]) marks the object changed; SetEpoch clears the marks and
// names a base. A State cloned from the base carries the base's epoch and
// its marks, so in any clone an object without a mark is, field for field,
// the object the base held: a reader that computed something from the base
// object reuses it. ObjHash is an object's content hash, a pure function of
// its fields (a clone of a different search world that holds the same
// object hashes the same, which is what lets an evaluation cache share
// entries across worlds; a counter-based stamp could not). ViewHash
// combines the hashes of everything a seat can see into the key of the
// position as that seat's network inputs read it.
//
// Marks are a superset: marking an unchanged object only costs a reader a
// recomputation. A missing mark is a stale reuse, so every mutation site
// marks (touch / touchSl), and TestMarksCoverEveryMutation checks after
// every action of a soak that each unmarked object still hashes as it did.

// touch marks object o changed and stamps it with the next touch count.
func (s *State) touch(o Obj) {
	s.chg[o>>6] |= 1 << (o & 63)
	s.touchN++
	s.stamp[o] = uint8(s.touchN)
}

// touchSl marks the object at battlefield slot sl changed.
func (s *State) touchSl(sl Slot) { s.touch(s.perms[sl].obj) }

// stashSet and stashClr give object o Tinybones' stash counter or take it
// away, marking o (ObjHash reads the bit).
func (s *State) stashSet(k *prep, o Obj) { s.mObjSet(k, mechStash, o); s.touch(o) }

func (s *State) stashClr(k *prep, o Obj) {
	if s.mObj(k, mechStash, o) {
		s.mObjClr(k, mechStash, o)
		s.touch(o)
	}
}

// Touches is the number of object writes since the State began (a
// counter that only grows along one line of play, cloned with the State).
func (s *State) Touches() uint32 { return s.touchN }

// SetLine names the State's line of play: a caller that branches a State
// off anything but its own forward play (a search's fresh world, a
// restored snapshot) gives it a new line, so a reader that caches per
// object along a line (ChangedSince) never continues a cache across lines.
func (s *State) SetLine(id uint64) { s.line = id }

// Undos counts the answers the State took back, returning to the position
// before them (choosing an unpayable either-or part undoes the cast): a
// search sees an edge that changed it as a no-op.
func (s *State) Undos() uint8 { return s.undos }

// Line is the State's line of play (SetLine).
func (s *State) Line() uint64 { return s.line }

// ChangedSince reports whether object o may have been written after the
// State's touch count was since (a count Touches returned earlier on this
// same line of play: this State or an ancestor it was cloned from, with no
// other line in between). The stamp keeps the low 8 bits of the count at
// the object's last write, so the answer is exact while fewer than 256
// writes happened since, and true for every object beyond that.
func (s *State) ChangedSince(o Obj, since uint32) bool {
	d := s.touchN - since
	if d == 0 {
		return false
	}
	if d >= 256 {
		return true
	}
	return uint32(uint8(s.stamp[o]-uint8(since)))-1 < d
}

// SetEpoch clears every change mark and records e as the State's epoch: the
// base an incremental reader computed its per-object results from. The
// touch count and stamps (ChangedSince) carry on.
func (s *State) SetEpoch(e uint32) {
	s.chg = [len(s.chg)]uint64{}
	s.epoch = e
}

// Epoch is the epoch SetEpoch last recorded (0 if never).
func (s *State) Epoch() uint32 { return s.epoch }

// Changed reports whether object o was written since SetEpoch.
func (s *State) Changed(o Obj) bool { return s.chg[o>>6]&(1<<(o&63)) != 0 }

// ChangedMask is the change marks as a bitset over objects.
func (s *State) ChangedMask() [ObjWords]uint64 { return s.chg }

func hmix(h, x uint64) uint64 {
	h ^= x
	h *= 0x9e3779b97f4a7c15
	return h ^ h>>29
}

// ObjHash is object o's content hash: its object fields and, on the
// battlefield, its slot's permanent fields.
func (s *State) ObjHash(o Obj) uint64 {
	ob := &s.objs[o]
	// every field but ver (unread by the encoder) and the in-hand history bit
	h := hmix(0x6f626a6563740002, uint64(ob.card)|uint64(ob.owner())<<16|uint64(ob.zone)<<24|uint64(ob.slot)<<32|
		uint64(ob.activations())<<48|uint64(ob.of&^ofInHand)<<56)
	if s.g.on && (ob.zone == kop.ZoneExile || ob.zone == kop.ZoneBattlefield) && s.mObj(s.k(), mechStash, o) {
		h = hmix(h, 0x7374617368) // Tinybones' stash counter (the view lists it: exiled cards and lands played from exile)
	}
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return h
	}
	// the permanent, every byte (perm has no padding)
	h = hbytes(h, unsafe.Pointer(&s.perms[ob.slot]), unsafe.Sizeof(s.perms[0]))
	if s.perms[ob.slot].ck[ctrSlots-1] != 0 {
		h = s.ctrOverHash(h, o)
	}
	if s.g.nlA[0] != 0 && s.netLocked(o) {
		h = hmix(h, 0x6e65746c6f636b) // Braided Net's lock (act_lockpermanent.go)
	}
	for i := 0; i < int(s.nkwx); i++ {
		if r := &s.kwx[i]; r.sl == ob.slot {
			// a keyword's until-end-of-turn grants past the third (the view
			// lists each one)
			h = hmix(h, 0x6b777800<<16|uint64(r.bit)<<8|uint64(r.n))
		}
	}
	for i := 0; i < int(s.nptx); i++ {
		if r := &s.ptx[i]; r.sl == ob.slot {
			h = hmix(h, 0x707478<<32|uint64(uint16(r.p))<<16|uint64(uint16(r.t))) // a pump past int8 (perm holds it saturated)
		}
	}
	if s.hasOvRec(ob.slot) || s.fxM.has(ob.slot) && s.copyExcHash(ob.slot) != 0 {
		// a generated table's override records and extra subtype on the slot
		r := s.ovRecs(ob.slot)
		h = hbytes(h, unsafe.Pointer(&r[0]), unsafe.Sizeof(r[0])*4)
		if r[4].idx != 0 {
			h = hbytes(h, unsafe.Pointer(&r[4]), unsafe.Sizeof(r[0])*4)
		}
		h = hmix(h, s.copyExcHash(ob.slot))
	}
	return h
}

// genViewHash mixes the generated-table state every derived
// characteristic may read into h: emblems, the turn's death and attack
// counts, the once-per-turn record.
func (s *State) genViewHash(h uint64) uint64 {
	g := &s.g
	if !g.on {
		return h
	}
	h = hbytes(h, unsafe.Pointer(&g.emb[0]), uintptr(len(g.emb))*unsafe.Sizeof(g.emb[0]))
	h = hbytes(h, unsafe.Pointer(&g.embC[0]), uintptr(len(g.embC))*unsafe.Sizeof(g.embC[0]))
	h = hbytes(h, unsafe.Pointer(&g.embO[0]), uintptr(len(g.embO))*unsafe.Sizeof(g.embO[0]))
	h = hbytes(h, unsafe.Pointer(&g.embN[0]), uintptr(len(g.embN))*unsafe.Sizeof(g.embN[0])) // the frozen chosen number
	h = hbytes(h, unsafe.Pointer(&g.embT[0]), uintptr(len(g.embT))*unsafe.Sizeof(g.embT[0])) // the layer timestamp
	h = hmix(h, uint64(g.nemb[0])|uint64(g.nemb[1])<<8|uint64(g.died)<<16|uint64(g.attacked[0])<<24|
		uint64(g.attacked[1])<<32|uint64(g.nused)<<40|uint64(g.oppAtt)<<48)
	if g.spent != [MaxSeats]uint16{} || g.hist != [MaxSeats][3]uint8{} {
		// the turn history bank: only a table that reads it keeps it
		h = hbytes(h, unsafe.Pointer(&g.hist), unsafe.Sizeof(g.hist))
		for p := uint8(0); p < 2; p++ {
			if r := s.castPushes(p); len(r) > 0 {
				h = hbytes(h, unsafe.Pointer(&r[0]), uintptr(len(r))*unsafe.Sizeof(r[0]))
			}
		}
		h = hmix(h, uint64(g.spent[0])|uint64(g.spent[1])<<16)
	}
	if g.crime != 0 {
		h = hmix(h, 0x6372696d65<<8|uint64(g.crime))
	}
	if g.colG != [maxColG]colGrant{} {
		h = hbytes(h, unsafe.Pointer(&g.colG), unsafe.Sizeof(g.colG)) // static_colourgrant.go
	}
	if s.exhAny() {
		h = hbytes(h, unsafe.Pointer(&s.cold.exH[0]), uintptr(len(s.cold.exH))*unsafe.Sizeof(s.cold.exH[0])) // static_cantblockexcept.go
	}
	if g.dayNight != 0 {
		h = hmix(h, 0x646e<<8|uint64(g.dayNight)) // CR 731 (kw_daynight.go)
	}
	if g.atkHi != 0 {
		h = s.atkGrpHash(h) // trig_attackers.go
	}
	return hbytes(h, unsafe.Pointer(&g.used[0]), uintptr(g.nused)*unsafe.Sizeof(g.used[0]))
}

// ViewHash is the hash of everything seat can see in s, in the orders it
// sees it: the turn, step, active and priority players, the combat's
// first-strike step, both players' life, zone sizes and mana pools; seat's
// hand, both graveyards and both exiles in zone order, the stack's entries,
// and every visible object's ObjHash (battlefield, seat's hand, graveyards,
// exiles, the stack). It is what dirac/feat's encoding of s from seat's
// view reads of s (the decision and its options are the caller's to add).
//
// base, when non-nil, holds ObjHash of every object of the State whose
// epoch s carries, taken when SetEpoch was called: an unmarked object's
// hash is read from it instead of recomputed.
func (s *State) ViewHash(seat uint8, base *[MaxObj]uint64) uint64 {
	return s.ViewHashCached(seat, base, nil)
}

// ViewHashCached is ViewHash reading a changed object's hash from c when
// it was computed on this line of play and the object is unchanged since,
// and the whole object part from c when no object was written since it was
// last computed for seat on this line.
func (s *State) ViewHashCached(seat uint8, base *[MaxObj]uint64, c *ObjHashCache) uint64 {
	if c != nil && (c.line != s.line || s.line == 0) {
		c.line = s.line
		c.ok = [MaxObj]bool{}
		c.part = [2]objPart{}
	}
	var o uint64
	if pc := c; pc != nil && s.line != 0 && pc.part[seat&1].ok && pc.part[seat&1].at == s.touchN {
		o = pc.part[seat&1].h
	} else {
		o = s.viewObjects(seat, base, c)
		if c != nil && s.line != 0 {
			c.part[seat&1] = objPart{ok: true, at: s.touchN, h: o}
		}
	}
	var fl uint64
	if s.fsStep {
		fl |= 1
	}
	if s.over {
		fl |= 2
	}
	h := hmix(0x7669657768617368, uint64(s.turn)|uint64(s.step)<<16|uint64(s.active)<<24|uint64(s.prio)<<32|
		uint64(seat)<<40|fl<<48)
	for p := 0; p < 2; p++ {
		pl := &s.pl[p]
		h = hmix(h, uint64(uint16(pl.life))|uint64(pl.handN)<<16|uint64(pl.libLen())<<24|uint64(pl.graveN)<<32|
			uint64(pl.exileN)<<40)
		var m uint64
		for i, n := range pl.pool {
			m |= uint64(n) << (8 * i)
		}
		h = hmix(h, m)
		if rh, ok := s.rbHash(uint8(p)); ok {
			h = hmix(h, rh) // restricted mana (mana_restrbatch.go)
		}
		if s.fbR[p] != 0 {
			h = hmix(h, uint64(s.fbR[p])<<8|0xfb) // firebending red (kw_firebend.go)
		}
		if s.pmOn>>p&1 != 0 {
			var m uint64
			for i, n := range s.pm[p] {
				m |= uint64(n) << (8 * i)
			}
			h = hmix(h, m^0xfa<<56) // PersistentMana$ tally (mana_persistent.go)
		}
	}
	for i := 0; i < int(s.sp); i++ {
		e := &s.stack[i]
		h = hmix(h, uint64(e.kind)|uint64(e.obj)<<8|uint64(e.card)<<16|uint64(e.ab)<<32|uint64(e.ctrl)<<40|
			uint64(e.mode)<<48|uint64(e.x)<<56)
		var t uint64
		for j, x := range e.tgt {
			t |= uint64(x) << (8 * (j & 7))
		}
		h = hmix(h, t|uint64(e.flags)<<56)
		if e.xi != 0 {
			h = hbytes(h, unsafe.Pointer(s.xtOf(e)), unsafe.Sizeof(noXT))
		}
	}
	return hmix(s.genViewHash(h), o)
}

// viewObjects is ViewHash's object part: every object seat sees, in the
// orders it sees them (seat's hand, the graveyards, the exiles, the
// stack's spells, the battlefield by slot), each by its ObjHash. Only an
// object write changes it.
func (s *State) viewObjects(seat uint8, base *[MaxObj]uint64, c *ObjHashCache) uint64 {
	oh := func(o Obj) uint64 {
		if base != nil && s.chg[o>>6]&(1<<(o&63)) == 0 {
			return base[o]
		}
		if c == nil || s.line == 0 {
			return s.ObjHash(o)
		}
		if c.ok[o] && !s.ChangedSince(o, c.at[o]) {
			return c.h[o]
		}
		h := s.ObjHash(o)
		c.ok[o], c.at[o], c.h[o] = true, s.touchN, h
		return h
	}
	h := uint64(0x6f626a7300)
	pl := &s.pl[seat&1]
	for i := 0; i < int(pl.handN); i++ {
		h = hmix(h, oh(pl.hand[i]))
	}
	for p := 0; p < 2; p++ {
		pl := &s.pl[p]
		h = hmix(h, 0x67)
		for i := 0; i < int(pl.graveN); i++ {
			h = hmix(h, oh(pl.grave[i]))
		}
		h = hmix(h, 0x65)
		for i := 0; i < int(pl.exileN); i++ {
			h = hmix(h, oh(pl.exile[i]))
		}
	}
	h = hmix(h, 0x73)
	for i := 0; i < int(s.sp); i++ {
		if o := s.stack[i].obj; o != NoObj && int(o) < int(s.nobj) && s.objs[o].zone == kop.ZoneStack {
			h = hmix(h, oh(o))
		}
	}
	h = hmix(h, 0x62)
	for m := s.occ; m.any(); m = m.rest() {
		h = hmix(h, oh(s.perms[m.first()].obj))
	}
	return h
}

// ObjHashCache keeps changed objects' ObjHash along one line of play
// (ViewHashCached), each valid until the object's next write, and each
// seat's object part while no object is written.
type ObjHashCache struct {
	line uint64
	ok   [MaxObj]bool
	at   [MaxObj]uint32 // Touches() when computed
	h    [MaxObj]uint64
	part [2]objPart
}

type objPart struct {
	ok bool
	at uint32
	h  uint64
}

// hbytes mixes the n bytes at p (a pointer-free value of the State) into h,
// eight at a time. Padding is hashed too: it can only make equal contents
// hash apart (a cache miss), never the reverse.
func hbytes(h uint64, p unsafe.Pointer, n uintptr) uint64 {
	b := unsafe.Slice((*byte)(p), n)
	for len(b) >= 8 {
		h = hmix(h, binary.LittleEndian.Uint64(b))
		b = b[8:]
	}
	var t uint64
	for i, x := range b {
		t |= uint64(x) << (8 * i)
	}
	return hmix(h, t)
}

// zoneIndex is o's position in its owner's hand, library (from the top),
// graveyard or exile; 255 elsewhere.
func (s *State) zoneIndex(o Obj) uint8 {
	ob := &s.objs[o]
	pl := &s.pl[ob.owner()&1]
	var l []Obj
	switch ob.zone {
	case kop.ZoneHand:
		l = pl.hand[:pl.handN]
	case kop.ZoneLibrary:
		l = pl.lib[pl.libLo:pl.libHi]
	case kop.ZoneGraveyard:
		l = pl.grave[:pl.graveN]
	case kop.ZoneExile:
		l = pl.exile[:pl.exileN]
	}
	for i, x := range l {
		if x == o {
			return uint8(i)
		}
	}
	return 255
}

// DecisionHash is the hash of decision d and the State it is posed in
// beyond ViewHash: the decision's kind, sub-kind, player and options; for
// each object an option names in a hidden zone, its zone, owner and
// position there (gorge's option list orders by them); the control state
// (the spell or ability being put on the stack, the resolution in
// progress, pending triggers and the trigger order owed, the mulligan,
// legend-rule and damage-division rounds, the cleanup discard). With
// ViewHash of the deciding seat it keys everything dirac/feat's option
// encoding reads.
func (s *State) DecisionHash(d *Decision) uint64 {
	h := hmix(0x6465636973696f6e, uint64(d.Kind)|uint64(d.Sub)<<8|uint64(d.Player)<<16|uint64(d.N)<<24|uint64(s.ctl)<<40)
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		h = hmix(h, uint64(a))
		for _, o := range [2]uint8{a.Source(), a.Target()} {
			if int(o) >= int(s.nobj) {
				continue
			}
			switch z := s.objs[o].zone; z {
			case kop.ZoneHand, kop.ZoneLibrary:
				h = hmix(h, uint64(o)|uint64(z)<<8|uint64(s.objs[o].owner())<<16|uint64(s.zoneIndex(Obj(o)))<<24)
			}
		}
	}
	switch s.ctl {
	case ctlBuild:
		h = hbytes(h, unsafe.Pointer(s.bld()), unsafe.Sizeof(build{}))
		h = hbytes(h, unsafe.Pointer(s.xtOf(&s.bld().e)), unsafe.Sizeof(noXT))
	case ctlResolve:
		h = hbytes(h, unsafe.Pointer(&s.res), unsafe.Sizeof(s.res))
		h = hbytes(h, unsafe.Pointer(s.xtOf(&s.res.e)), unsafe.Sizeof(noXT))
	}
	if s.g.on {
		// a generated table's decision context: the multi-object target
		// slot being chosen, the pending as-enters asks, may-play
		// permissions
		g := &s.g
		h = hbytes(h, unsafe.Pointer(&g.mt), unsafe.Sizeof(g.mt))
		h = hbytes(h, unsafe.Pointer(&g.ask[0]), uintptr(len(g.ask))*unsafe.Sizeof(g.ask[0]))
		h = hbytes(h, unsafe.Pointer(&g.pre), unsafe.Sizeof(g.pre))
		if g.ro.on || g.ro.n > 0 {
			h = hbytes(h, unsafe.Pointer(&g.ro.replHdr), unsafe.Sizeof(g.ro.replHdr))
			h = hbytes(h, unsafe.Pointer(&g.nc), unsafe.Sizeof(g.nc))
			h = hbytes(h, unsafe.Pointer(&g.tok), unsafe.Sizeof(g.tok))
		}
		if g.ro.on {
			// arm A of scr (scratch.go): the frozen body and the
			// division it carries; a noncombat ask (ro.n > 0, not on) has
			// none, and the bytes are another arm's then
			h = hbytes(h, unsafe.Pointer(s.roBody()), unsafe.Sizeof(replBody{}))
			h = hbytes(h, unsafe.Pointer(&s.combatArm().divAmt), unsafe.Sizeof(s.combatArm().divAmt))
		}
		h = hbytes(h, unsafe.Pointer(&g.may[0]), uintptr(min(int(g.nmay), maxMay))*unsafe.Sizeof(g.may[0]))
		if cl := s.mayColdLive(); len(cl) > 0 {
			h = hbytes(h, unsafe.Pointer(&cl[0]), uintptr(len(cl))*unsafe.Sizeof(cl[0]))
		}
		h = hmix(h, uint64(g.nask)|uint64(g.ctlRet)<<8|uint64(g.nmay)<<16|uint64(g.nrem)<<24|uint64(g.nchosen)<<32|uint64(g.priSave)<<40|uint64(g.askPrio)<<48|uint64(g.holdPrio)<<56)
		h = hbytes(h, unsafe.Pointer(&g.rem[0]), uintptr(g.nrem)*unsafe.Sizeof(g.rem[0]))
		h = hbytes(h, unsafe.Pointer(&g.chosen[0]), uintptr(g.nchosen)*unsafe.Sizeof(g.chosen[0]))
		if g.nimp != 0 {
			h = hmix(h, uint64(g.nimp)) // linkImp joins the hash with the persistence that writes it
			h = hbytes(h, unsafe.Pointer(&g.imp[0]), uintptr(g.nimp)*unsafe.Sizeof(g.imp[0]))
		}
	}
	h = hbytes(h, unsafe.Pointer(&s.pend[0]), uintptr(s.npend)*unsafe.Sizeof(s.pend[0]))
	if s.g.atkHi != 0 {
		h = s.atkGrpHash(h) // the attacker groups the pending and stacked triggers index
	}
	h = hmix(h, uint64(s.npend)|uint64(s.orderLeft)<<8|uint64(s.orderN)<<16|uint64(s.orderPlace)<<24|
		uint64(s.discardLeft)<<32|uint64(s.legP)<<40|uint64(s.legCard)<<48)
	h = hmix(h, s.legDie.lo)
	h = hmix(h, s.legDie.hi)
	if s.legSaga.any() {
		h = hmix(hmix(h, s.legSaga.lo), s.legSaga.hi)
	}
	h = hbytes(h, unsafe.Pointer(&s.ordList[0]), uintptr(len(s.ordList))*unsafe.Sizeof(s.ordList[0]))
	h = hmix(h, uint64(s.mullOrder[0])|uint64(s.mullOrder[1])<<8|uint64(s.mullCur)<<16|uint64(s.mullLeft)<<24|
		uint64(s.mullMax)<<32|uint64(s.divA)<<40|uint64(s.divI)<<48|uint64(s.divRem)<<56)
	var fl uint64
	for i, b := range [5]bool{s.mullKept[0], s.mullKept[1], s.mullMull[0], s.mullMull[1], s.divFirst} {
		if b {
			fl |= 1 << i
		}
	}
	// arm A of scr: the division is read only while a combat damage pass
	// holds it (zero otherwise, as it was before the overlay)
	var dd slotMask
	if s.scrCombatLive() {
		dd = s.combatArm().divDone
	}
	h = hmix(h, dd.lo)
	h = hmix(h, dd.hi)
	h = hmix(h, fl)
	if s.ctl == ctlDivide {
		h = hbytes(h, unsafe.Pointer(&s.combatArm().divAmt), unsafe.Sizeof(s.combatArm().divAmt))
	}
	return h
}

// CheckMarks reports the first object of s that is unmarked although its
// ObjHash differs from base (the hashes taken at s's epoch): a mutation
// site that does not mark.
func CheckMarks(s *State, base *[MaxObj]uint64) (Obj, bool) {
	// (a test helper, exported so the oracle's FDN soak can run it)
	for o := 0; o < int(s.nobj); o++ {
		if !s.Changed(Obj(o)) && s.ObjHash(Obj(o)) != base[o] {
			return Obj(o), false
		}
	}
	return 0, true
}
