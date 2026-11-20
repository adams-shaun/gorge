package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A creature with a printed "can block an additional creature each combat"
// (CardDef.ExtraBlocks, CR 509.1a) is declared against at most two
// attackers, and one that "can block any number of creatures"
// (kop.ExtraBlocksAny, combat_blockany.go) against as many as the table
// holds. perm.blocking and perm.bord keep the FIRST attacker and its
// declaration order; every further one lives in the cold table xblkRecs, a
// record each, sized only for a pool with genPrep.anyExtraBlock (a nil table
// reads as "no further attacker", so every other pool pays one length
// check). perm.nblock counts blockers per attacker, so a blocker of several
// attackers adds one to each.
//
// Every reader of "what does this blocker block" goes through the accessors
// here instead of reading perm.blocking.

// xblkRec is one of a blocker's further blocks (combat_extrablock.go).
// b: the blocker's slot. a: the attacker's slot + 1 (0: a free record);
// ord: blockSeq at that pick (the pair's declaration order, beside perm.bord
// for the first); div: the damage the blocker's controller divided to this
// attacker, meaningful only while combatArm().divDone.has(blocker) (the first
// attacker's share is what is left of the blocker's power); adiv: the damage
// this attacker's division gave the blocker (divAmt holds the first
// attacker's), meaningful only while that attacker's divDone is set.
type xblkRec struct{ b, a, ord, div, adiv uint8 }

// blkPair is one declared (blocker, attacker) pair.
type blkPair struct{ b, a Slot }

// maxXblk is how many further blocks one combat holds records for in a pool
// with an extra-block source but no CanBlockAny$ one. The table is small
// because the mbc pool sits against the State bar (TestEveryPoolFitsTheStateBar);
// a blocker whose further block would need a ninth record is not offered one
// (StallExtraBlockCap). A pool with a CanBlockAny$ source holds maxXblkAny.
const maxXblk = 8

// xblkRecs is the further-block table (nil when the pool has no ExtraBlocks
// card), a record to each further attacker of a blocker, found by its b. It
// sits in the cold buffer, so State itself does not grow (TestStateLayoutPinned).
// A slot's records are dropped as the slot takes a new permanent (xblkDrop,
// from arenaClrSlot).
func (s *State) xblkRecs() []xblkRec { return coldXblk.of(s) }

var coldXblk = newCold[xblkRec]("xblk", func(g *genPrep) int {
	switch {
	case g.uses.has(useBlockAny):
		return maxXblkAny
	case g.anyExtraBlock:
		return maxXblk
	}
	return 0
})

// xblkFind is blocker b's first further-block record (nil when it has none).
func (s *State) xblkFind(b Slot) *xblkRec {
	r := s.xblkRecs()
	for i := range r {
		if r[i].a != 0 && r[i].b == uint8(b) {
			return &r[i]
		}
	}
	return nil
}

// xblkAt is the record of b's block of attacker a (nil when a is not one of
// b's further attackers).
func (s *State) xblkAt(b, a Slot) *xblkRec {
	r := s.xblkRecs()
	for i := range r {
		if r[i].a == uint8(a)+1 && r[i].b == uint8(b) {
			return &r[i]
		}
	}
	return nil
}

// xblkHash is blocker b's further-block records folded into one word (0: none),
// by a sum so the table position of a record does not matter.
func (s *State) xblkHash(b Slot) uint64 {
	var h uint64
	for _, x := range s.xblkRecs() {
		if x.a != 0 && x.b == uint8(b) {
			h += hmix(0x78626c6b, uint64(x.a)<<16|uint64(x.ord)<<8|uint64(x.div)|uint64(x.adiv)<<56)
		}
	}
	return h
}

// xblkDrop ends every further block of blocker b.
func (s *State) xblkDrop(b Slot) {
	r := s.xblkRecs()
	for i := range r {
		if r[i].a != 0 && r[i].b == uint8(b) {
			r[i] = xblkRec{}
		}
	}
}

// xblkRoom: the table has a free record.
func (s *State) xblkRoom() bool {
	for _, r := range s.xblkRecs() {
		if r.a == 0 {
			return true
		}
	}
	return false
}

// xblkAdd records attacker at as one of blocker b's further blocks, in declaration
// order ord. It reports false when the table is full.
func (s *State) xblkAdd(b, at Slot, ord uint8) bool {
	r := s.xblkRecs()
	for i := range r {
		if r := &r[i]; r.a == 0 {
			*r = xblkRec{b: uint8(b), a: uint8(at) + 1, ord: ord}
			return true
		}
	}
	return false
}

// xblkOf is blocker b's first further attacker (NoSlot none).
func (s *State) xblkOf(b Slot) Slot {
	if r := s.xblkFind(b); r != nil {
		return Slot(r.a - 1)
	}
	return NoSlot
}

// blocks: b is blocking attacker a.
func (s *State) blocks(b, a Slot) bool {
	p := &s.perms[b]
	return p.flags&pfBlocking != 0 && (p.blocking == a || s.xblkAt(b, a) != nil)
}

// blockKey is the declaration order of the (b, a) pair: perm.bord for b's
// first attacker, xblk.ord for a further one.
func (s *State) blockKey(b, a Slot) uint8 {
	if p := &s.perms[b]; p.blocking != a {
		if r := s.xblkAt(b, a); r != nil {
			return r.ord
		}
	}
	return uint8(s.perms[b].bord)
}

// blockPairs writes every declared pair of the defenders' blockers, in
// declaration order, and returns how many.
func (s *State) blockPairs(out *[2 * MaxPerm]blkPair) int {
	var key [2 * MaxPerm]uint8
	n := 0
	add := func(b, a Slot, o uint8) {
		j := n
		for j > 0 && key[j-1] > o {
			out[j], key[j] = out[j-1], key[j-1]
			j--
		}
		out[j], key[j] = blkPair{b, a}, o
		n++
	}
	for m := s.oppsM(s.active); m.any(); m = m.rest() {
		b := m.first()
		p := &s.perms[b]
		if p.flags&pfBlocking == 0 || p.blocking == NoSlot {
			continue
		}
		add(b, p.blocking, uint8(p.bord))
		for _, r := range s.xblkRecs() {
			if r.a != 0 && r.b == uint8(b) {
				add(b, Slot(r.a-1), r.ord)
			}
		}
	}
	return n
}

// blockedLive writes the attackers b blocks that still attack into out,
// ordered by ts, the order its damage division lists them (gorge
// blockedAttackers is battlefield order, which ts equates), and returns how
// many.
func (s *State) blockedLive(b Slot, out *[MaxPerm]Slot) int {
	n := 0
	add := func(a Slot) {
		if s.perms[a].flags&pfAttacking == 0 {
			return
		}
		j := n
		for j > 0 && s.perms[out[j-1]].ts > s.perms[a].ts {
			out[j] = out[j-1]
			j--
		}
		out[j] = a
		n++
	}
	if a := s.perms[b].blocking; a != NoSlot {
		add(a)
	}
	for _, r := range s.xblkRecs() {
		if r.a != 0 && r.b == uint8(b) {
			add(Slot(r.a - 1))
		}
	}
	return n
}

// forgetBlocked: attacker a left blocker b's blocks. b stays blocking
// (CR 506.4) and blocks what is left. It reports whether b blocked a.
func (s *State) forgetBlocked(b, a Slot) bool {
	p := &s.perms[b]
	if r := s.xblkAt(b, a); r != nil {
		*r = xblkRec{}
	} else if p.blocking == a {
		p.blocking = NoSlot
		if r := s.xblkEarliest(b); r != nil { // the earliest declared further block becomes the first
			p.blocking, p.bord = Slot(r.a-1), objWord(r.ord)
			*r = xblkRec{}
		}
	} else {
		return false
	}
	s.touchSl(b)
	return true
}

// xblkEarliest is b's further-block record with the lowest declaration order.
func (s *State) xblkEarliest(b Slot) *xblkRec {
	var best *xblkRec
	r := s.xblkRecs()
	for i := range r {
		if r[i].a != 0 && r[i].b == uint8(b) && (best == nil || r[i].ord < best.ord) {
			best = &r[i]
		}
	}
	return best
}

// releaseBlocked: blocker b left combat. Each attacker it blocked has one
// blocker fewer.
func (s *State) releaseBlocked(b Slot) {
	p := &s.perms[b]
	if p.blocking != NoSlot {
		s.dropBlocker(p.blocking)
	}
	for _, r := range s.xblkRecs() {
		if r.a != 0 && r.b == uint8(b) {
			s.dropBlocker(Slot(r.a - 1))
		}
	}
	s.xblkDrop(b)
}

func (s *State) dropBlocker(a Slot) {
	if ap := &s.perms[a]; ap.nblock > 0 {
		ap.nblock--
		s.touchSl(a)
	}
}

// blockProjected is the attacker gorge's projection names as what b blocks:
// the lowest-instance one of its attackers (NoSlot when it blocks nothing).
func (s *State) blockProjected(b Slot) Slot {
	a := s.perms[b].blocking
	for _, r := range s.xblkRecs() {
		if x := Slot(r.a - 1); r.a != 0 && r.b == uint8(b) && (a == NoSlot || s.perms[x].obj < s.perms[a].obj) {
			a = x
		}
	}
	return a
}

// xblkClear drops every second-block record (combat ends).
func (s *State) xblkClear() {
	clear(s.xblkRecs())
}

// StallExtraBlockReq: a requirement- or lure-maximising block declaration with
// a two-attacker blocker ready. No longer reached: combat_mustbeblocked.go's
// search offers the second block there (the number stays, stallNames and the
// stall classes read it).
const StallExtraBlockReq = 163

func init() { stallNames[StallExtraBlockReq] = "extrablockreq" }

// StallExtraBlockCap: a blocker that two CanBlockAmount$ 1 sources reach (a
// printed one on itself and a grant, or two grants) and that already blocks two
// attackers while a third is blockable: gorge's BlockCapacity sums the sources
// to three attackers, and the kernel's second-block record holds two.
// It is also the stall of a ninth blocker of two attackers in one combat: the
// second-block table holds maxXblk records.
const StallExtraBlockCap = 192

func init() { stallNames[StallExtraBlockCap] = "extrablockcap" }

// blockSources counts the CanBlockAmount$ 1 sources that reach b (CR 509.1a): a
// printed one on itself unless it has lost its abilities, and each
// StatExtraBlock grant. Each adds one to gorge's BlockCapacity.
func (s *State) blockSources(k *prep, b Slot) int {
	if k.gen == nil || !k.gen.anyExtraBlock {
		return 0
	}
	n := 0
	if x := k.t.Cards[s.cardAt(b)].ExtraBlocks; x != 0 && x != kop.ExtraBlocksAny && !s.genLost(k, b) {
		n++
	}
	n += s.extraBlockGrants(k, b)
	return n
}

// blockCap is how many attackers b may block (CR 509.1a), as far as the
// kernel's records hold: one, two with any extra-block source, or blockCapAny
// with a CanBlockAny$ one (combat_blockany.go). A second CanBlockAmount$ 1
// source makes gorge's capacity three; blockerRoom stalls the game when that
// third block could be offered (StallExtraBlockCap).
func (s *State) blockCap(k *prep, b Slot) int {
	if s.blockAny(k, b) {
		return blockCapAny
	}
	if s.blockSources(k, b) > 0 {
		return 2
	}
	return 1
}

// thirdBlock: the blocker b of two attackers could be declared against a
// third (some other attacker it can block), which gorge offers under a
// capacity of three.
//
//go:noinline
func (s *State) thirdBlock(k *prep, b Slot) bool {
	for am := s.blockAtkM(k); am.any(); am = am.rest() {
		a := am.first()
		if s.perms[a].flags&pfAttacking != 0 && !s.blocks(b, a) && s.canBlock(k, b, a) {
			return true
		}
	}
	return false
}

// extraBlockGrants counts the StatExtraBlock statics whose filter matches
// blocker b (gorge BlockCapacity adds each matching static's amount).
//
//go:noinline
func (s *State) extraBlockGrants(k *prep, b Slot) int {
	n := 0
	s.eachStatic(k, func(st statSrc) bool {
		if a := st.a; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatExtraBlock && !isBlockAnyGrant(a) && s.statCond(k, st) &&
			s.matchPermGen(k, a.Static.Filter, b, st.ctrl, st.sl, layerFull) {
			n++
		}
		return true
	})
	return n
}

// blockerRoom: b may be declared against an attacker now: a ready blocker, or
// one that blocks a single attacker and has room for a second.
func (s *State) blockerRoom(k *prep, b Slot) bool {
	if s.blockerReady(k, b) {
		return true
	}
	p := &s.perms[b]
	if p.flags&pfBlocking == 0 || p.flags&pfTapped != 0 || !s.isCreature(k, b) || s.kw(k, b)&kop.KwCantBlock != 0 {
		return false
	}
	if s.blockAny(k, b) {
		return s.blockAnyRoom(k, b) // combat_blockany.go
	}
	if s.xblkOf(b) != NoSlot {
		if s.blockSources(k, b) > 1 && s.thirdBlock(k, b) {
			s.stall(StallExtraBlockCap) // capacity three: the kernel holds two
		}
		return false
	}
	if s.blockCap(k, b) != 2 {
		return false
	}
	if !s.xblkRoom() {
		s.stall(StallExtraBlockCap) // the table holds maxXblk blockers of two
		return false
	}
	return true
}

// blockedMany: b blocks two or more attackers that still attack.
func (s *State) blockedMany(b Slot) bool {
	var l [MaxPerm]Slot
	return s.perms[b].blocking != NoSlot && s.blockedLive(b, &l) > 1
}

// xblkDivReset zeroes every blocker division share (the pass's divDone is gone).
func (s *State) xblkDivReset() {
	r := s.xblkRecs()
	for i := range r {
		r[i].div, r[i].adiv = 0, 0
	}
}

// divAmtFor is where attacker a's division share to blocker b is kept: divAmt[b]
// for b's first attacker, xblk.adiv for a further one, so the attackers'
// divisions do not overwrite each other.
func (s *State) divAmtFor(b, a Slot) *uint8 {
	if s.perms[b].blocking != a {
		if r := s.xblkAt(b, a); r != nil {
			return &r.adiv
		}
	}
	return &s.combatArm().divAmt[b]
}

// blockerMultiDamage is blocker b's combat damage of pw to its attackers
// (CR 510.1d): its controller's chosen division when one was asked, else
// gorge's bandBlockerAssignments fallback (lethal to each attacker in order,
// the rest to the last). Notes go in ts order, the order gorge lists them.
func (s *State) blockerMultiDamage(k *prep, r *dmgRec, b Slot, pw int) {
	var l [MaxPerm]Slot
	n := s.blockedLive(b, &l)
	dt := s.kw(k, b)&kop.KwDeathtouch != 0
	chosen := s.combatArm().divDone.has(b)
	rem := pw
	if chosen { // the first attacker's share is what the further ones leave
		for _, a := range l[:n] {
			if a != s.perms[b].blocking {
				rem -= int(s.xblkAt(b, a).div)
			}
		}
	}
	left := pw
	for i, a := range l[:n] {
		var give int
		switch {
		case chosen && a == s.perms[b].blocking:
			give = rem
		case chosen:
			give = int(s.xblkAt(b, a).div)
		default:
			give = left
			if i < n-1 {
				_, t := s.pt(k, a)
				lethal := t - int(s.perms[a].damage) // marked damage only, as gorge lethalDamageNeed
				if dt {
					if s.perms[a].flags&pfDeathtouched != 0 {
						lethal = 0
					} else if lethal > 1 {
						lethal = 1
					}
				}
				give = min(give, max(lethal, 0))
			}
		}
		give = min(max(give, 0), left)
		left -= give
		if give <= 0 {
			continue
		}
		r.toPerm[a] = s.addSat16(r.toPerm[a], give)
		if dt {
			r.dtPerm.set(a)
		}
		r.dealt[b] = s.addSat16(r.dealt[b], give)
		r.note(b, a, s.sat16(give))
	}
}

// nextBlockerDivision finds the next blocker of several attackers whose damage
// its controller divides, after every attacker division of the pass:
// attackers in ts order (order[:no]), each one's live blockers in block order
// (gorge divisionNeeding -> appendBandBlockerDivisions).
func (s *State) nextBlockerDivision(k *prep, first bool, order *[MaxPerm]Slot, no int) bool {
	for oi := 0; oi < no; oi++ {
		var bl [MaxPerm]Slot
		nb := s.liveBlockers(order[oi], &bl)
		for _, b := range bl[:nb] {
			if !s.blockedMany(b) || s.combatArm().divDone.has(b) || !s.dealsNow(k, b, first) {
				continue
			}
			pw := s.combatAmount(k, b)
			var l [MaxPerm]Slot
			if pw <= 0 || splits(pw, s.blockedLive(b, &l)) > 128 {
				continue
			}
			s.divA, s.divI, s.divRem = b, 0, uint8(pw)
			return true
		}
	}
	return false
}

// blockerDivideOptions poses the division of a multi-attacker blocker's
// damage to its controller: one option per split, ascending in the first
// attacker's share (SplitAt with one recipient per live attacker, in ts order).
func (s *State) blockerDivideOptions(d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChDivide, s.perms[s.divA].ctrl
	o := s.perms[s.divA].obj
	var l [MaxPerm]Slot
	for i, n := 0, splits(int(s.divRem), s.blockedLive(s.divA, &l)); i < n; i++ {
		d.add(MakeAction(AkChoose, TCode(o), 0, uint8(i)))
	}
}

// applyBlockerDivide records the controller's division of blocker divA's
// damage among its attackers, one share a further attacker's record.
func (s *State) applyBlockerDivide(a Action) {
	var l [MaxPerm]Slot
	n := s.blockedLive(s.divA, &l)
	var amt [MaxPerm]uint8
	SplitAt(int(s.divRem), n, int(a.Mode()), amt[:n])
	if DebugExtraBlockDivideFirst {
		clear(amt[:n])
		amt[0] = s.divRem
	}
	for i, at := range l[:n] {
		if r := s.xblkAt(s.divA, at); r != nil && s.perms[s.divA].blocking != at {
			r.div = amt[i]
		}
	}
	s.combatArm().divDone.set(s.divA)
	s.divA = NoSlot
}

// A table with a StatExtraBlock grant takes a genPrep: anyExtraBlock is set in
// prepareGen (gen.go).
func init() {
	genNeed(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatExtraBlock) })
}

// DebugExtraBlockDivideFirst is the mutation hook (test only): a blocker of
// several attackers puts all its damage on the first of them, whatever its
// controller divided.
var DebugExtraBlockDivideFirst bool
