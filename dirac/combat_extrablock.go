package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A creature with a printed "can block an additional creature each combat"
// (CardDef.ExtraBlocks, CR 509.1a) is declared against at most two
// attackers. perm.blocking and perm.bord keep the FIRST attacker and its
// declaration order; the second lives in the cold table xblkRecs, sized
// only for a pool with genPrep.anyExtraBlock (a nil table reads as "no
// second attacker", so every other pool pays one length check). perm.nblock
// counts blockers per attacker, so a two-attacker blocker adds one to each.
//
// Every reader of "what does this blocker block" goes through the accessors
// here instead of reading perm.blocking.

// xblkRec is a two-attacker blocker's second block (combat_extrablock.go).
// b: the blocker's slot. a: the second attacker's slot + 1 (0: a free record);
// ord: blockSeq at that pick (the pair's declaration order, beside perm.bord
// for the first); div: the blocker's chosen damage to the lower-ts of its two
// attackers, meaningful only while combatArm().divDone.has(blocker); adiv: the
// damage the higher-ts attacker's division gave this blocker (divAmt holds the
// lower-ts attacker's), meaningful only while that attacker's divDone is set.
type xblkRec struct{ b, a, ord, div, adiv uint8 }

// blkPair is one declared (blocker, attacker) pair.
type blkPair struct{ b, a Slot }

// maxXblk is how many blockers of two attackers one combat holds records for.
// The table is small because the mbc pool sits against the State bar
// (TestEveryPoolFitsTheStateBar); a blocker whose second block would need a
// ninth record is not offered one (StallExtraBlockCap).
const maxXblk = 8

// xblkRecs is the second-block table (nil when the pool has no ExtraBlocks
// card), a record to a blocker of two attackers, found by its b. It sits in the
// cold buffer, so State itself does not grow (TestStateLayoutPinned). A slot's
// record is dropped as the slot takes a new permanent (xblkDrop, from
// arenaClrSlot).
func (s *State) xblkRecs() []xblkRec { return coldXblk.of(s) }

var coldXblk = newCold[xblkRec]("xblk", func(g *genPrep) int {
	if g.anyExtraBlock {
		return maxXblk
	}
	return 0
})

// xblkFind is blocker b's second-block record (nil when it has none).
func (s *State) xblkFind(b Slot) *xblkRec {
	r := s.xblkRecs()
	for i := range r {
		if r[i].a != 0 && r[i].b == uint8(b) {
			return &r[i]
		}
	}
	return nil
}

// xblkDrop ends blocker b's second block, if any.
func (s *State) xblkDrop(b Slot) {
	if r := s.xblkFind(b); r != nil {
		*r = xblkRec{}
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

// xblkAdd records attacker at as blocker b's second block, in declaration
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

// xblkOf is blocker b's second attacker (NoSlot none).
func (s *State) xblkOf(b Slot) Slot {
	if r := s.xblkFind(b); r != nil {
		return Slot(r.a - 1)
	}
	return NoSlot
}

// blocks: b is blocking attacker a.
func (s *State) blocks(b, a Slot) bool {
	p := &s.perms[b]
	return p.flags&pfBlocking != 0 && (p.blocking == a || s.xblkOf(b) == a)
}

// blockKey is the declaration order of the (b, a) pair: perm.bord for b's
// first attacker, xblk.ord for its second.
func (s *State) blockKey(b, a Slot) uint8 {
	if p := &s.perms[b]; p.blocking != a && s.xblkOf(b) == a {
		return s.xblkFind(b).ord
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
		if x := s.xblkOf(b); x != NoSlot {
			add(b, x, s.xblkFind(b).ord)
		}
	}
	return n
}

// blockedPair is b's two attackers ordered by ts, the order its damage
// division lists them (gorge blockedAttackers is battlefield order, which ts
// equates). a2 is NoSlot for a one-attacker blocker.
func (s *State) blockedPair(b Slot) (a1, a2 Slot) {
	a1, a2 = s.perms[b].blocking, s.xblkOf(b)
	if a2 != NoSlot && s.perms[a2].ts < s.perms[a1].ts {
		a1, a2 = a2, a1
	}
	return a1, a2
}

// forgetBlocked: attacker a left blocker b's blocks. b stays blocking
// (CR 506.4) and blocks what is left. It reports whether b blocked a.
func (s *State) forgetBlocked(b, a Slot) bool {
	p := &s.perms[b]
	switch {
	case s.xblkOf(b) == a:
		s.xblkDrop(b)
	case p.blocking == a:
		p.blocking = NoSlot
		if r := s.xblkFind(b); r != nil {
			p.blocking, p.bord = Slot(r.a-1), objWord(r.ord)
			*r = xblkRec{}
		}
	default:
		return false
	}
	s.touchSl(b)
	return true
}

// releaseBlocked: blocker b left combat. Each attacker it blocked has one
// blocker fewer.
func (s *State) releaseBlocked(b Slot) {
	p := &s.perms[b]
	if p.blocking != NoSlot {
		s.dropBlocker(p.blocking)
	}
	if x := s.xblkOf(b); x != NoSlot {
		s.dropBlocker(x)
		s.xblkDrop(b)
	}
}

func (s *State) dropBlocker(a Slot) {
	if ap := &s.perms[a]; ap.nblock > 0 {
		ap.nblock--
		s.touchSl(a)
	}
}

// blockProjected is the attacker gorge's projection names as what b blocks:
// the lower-instance one of its two (NoSlot when it blocks nothing).
func (s *State) blockProjected(b Slot) Slot {
	a := s.perms[b].blocking
	if x := s.xblkOf(b); x != NoSlot && (a == NoSlot || s.perms[x].obj < s.perms[a].obj) {
		return x
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
	if k.t.Cards[s.cardAt(b)].ExtraBlocks != 0 && !s.genLost(k, b) {
		n++
	}
	n += s.extraBlockGrants(k, b)
	return n
}

// blockCap is how many attackers b may block (CR 509.1a), as far as the
// kernel's records hold: one, or two with any extra-block source. A second
// source makes gorge's capacity three; blockerRoom stalls the game when that
// third block could be offered (StallExtraBlockCap).
func (s *State) blockCap(k *prep, b Slot) int {
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
		if a := st.a; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatExtraBlock && s.statCond(k, st) &&
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

// blockedTwo: b blocks two attackers that both still attack.
func (s *State) blockedTwo(b Slot) bool {
	x := s.xblkOf(b)
	return x != NoSlot && s.perms[x].flags&pfAttacking != 0 && s.perms[s.perms[b].blocking].flags&pfAttacking != 0
}

// xblkDivReset zeroes every blocker division share (the pass's divDone is gone).
func (s *State) xblkDivReset() {
	r := s.xblkRecs()
	for i := range r {
		r[i].div, r[i].adiv = 0, 0
	}
}

// divAmtFor is where attacker a's division share to blocker b is kept: divAmt[b],
// or xblk.adiv when b blocks two attackers and a is the later (ts) of them, so
// the two attackers' divisions do not overwrite each other.
func (s *State) divAmtFor(b, a Slot) *uint8 {
	if s.xblkOf(b) != NoSlot {
		if _, a2 := s.blockedPair(b); a == a2 {
			return &s.xblkFind(b).adiv
		}
	}
	return &s.combatArm().divAmt[b]
}

// blockerTwoDamage is blocker b's combat damage of pw to its two attackers
// (CR 510.1d): its controller's chosen split when one was asked, else gorge's
// bandBlockerAssignments fallback (lethal to the first, the rest to the
// second). Notes go first attacker then second.
func (s *State) blockerTwoDamage(k *prep, r *dmgRec, b Slot, pw int) {
	a1, a2 := s.blockedPair(b)
	dt := s.kw(k, b)&kop.KwDeathtouch != 0
	var n1 int
	if s.combatArm().divDone.has(b) {
		if xr := s.xblkFind(b); xr != nil {
			n1 = int(xr.div)
		}
	} else {
		_, t := s.pt(k, a1)
		lethal := t - int(s.perms[a1].damage) // marked damage only, as gorge lethalDamageNeed
		if dt {
			if s.perms[a1].flags&pfDeathtouched != 0 {
				lethal = 0
			} else if lethal > 1 {
				lethal = 1
			}
		}
		n1 = max(lethal, 0)
	}
	n1 = min(n1, pw)
	for i, n := range [2]int{n1, pw - n1} {
		if n <= 0 {
			continue
		}
		a := [2]Slot{a1, a2}[i]
		r.toPerm[a] = s.addSat16(r.toPerm[a], n)
		if dt {
			r.dtPerm.set(a)
		}
		r.dealt[b] = s.addSat16(r.dealt[b], n)
		r.note(b, a, s.sat16(n))
	}
}

// nextBlockerDivision finds the next two-attacker blocker whose damage its
// controller divides, after every attacker division of the pass: attackers in
// ts order (order[:no]), each one's live blockers in block order (gorge
// divisionNeeding -> appendBandBlockerDivisions).
func (s *State) nextBlockerDivision(k *prep, first bool, order *[MaxPerm]Slot, no int) bool {
	for oi := 0; oi < no; oi++ {
		var bl [MaxPerm]Slot
		nb := s.liveBlockers(order[oi], &bl)
		for _, b := range bl[:nb] {
			if !s.blockedTwo(b) || s.combatArm().divDone.has(b) || !s.dealsNow(k, b, first) {
				continue
			}
			pw := s.power(k, b)
			if k.gen != nil && k.gen.anyCdmgTough {
				pw = s.combatAmtTough(k, b)
			}
			if pw <= 0 || splits(pw, 2) > 128 {
				continue
			}
			s.divA, s.divI, s.divRem = b, 0, uint8(pw)
			return true
		}
	}
	return false
}

// blockerDivideOptions poses the division of a two-attacker blocker's damage
// to its controller: one option per split, ascending in the first attacker's
// share (SplitAt with two recipients).
func (s *State) blockerDivideOptions(d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChDivide, s.perms[s.divA].ctrl
	o := s.perms[s.divA].obj
	for i, n := 0, splits(int(s.divRem), 2); i < n; i++ {
		d.add(MakeAction(AkChoose, TCode(o), 0, uint8(i)))
	}
}

// applyBlockerDivide records the controller's split of blocker divA's damage.
func (s *State) applyBlockerDivide(a Action) {
	var amt [2]uint8
	SplitAt(int(s.divRem), 2, int(a.Mode()), amt[:])
	if r := s.xblkFind(s.divA); r != nil {
		r.div = amt[0]
	}
	s.combatArm().divDone.set(s.divA)
	s.divA = NoSlot
}
