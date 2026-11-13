package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A creature with a printed "can block an additional creature each combat"
// (CardDef.ExtraBlocks, CR 509.1a) is declared against at most two
// attackers. perm.blocking and perm.bord keep the FIRST attacker and its
// declaration order; the second lives in the cold array xblkRecs, sized
// only for a pool with genPrep.anyExtraBlock (a nil array reads as "no
// second attacker", so every other pool pays one length check). perm.nblock
// counts blockers per attacker, so a two-attacker blocker adds one to each.
//
// Every reader of "what does this blocker block" goes through the accessors
// here instead of reading perm.blocking.

// xblkRec is a two-attacker blocker's second block (combat_extrablock.go),
// indexed by the blocker's slot. a: the second attacker's slot + 1 (0 none);
// ord: blockSeq at that pick (the pair's declaration order, beside perm.bord
// for the first); div: the blocker's chosen damage to the lower-ts of its two
// attackers, meaningful only while combatArm().divDone.has(blocker); adiv: the
// damage the higher-ts attacker's division gave this blocker (divAmt holds the
// lower-ts attacker's), meaningful only while that attacker's divDone is set.
type xblkRec struct{ a, ord, div, adiv uint8 }

// blkPair is one declared (blocker, attacker) pair.
type blkPair struct{ b, a Slot }

// xblkRecs is the second-block records by blocker slot (nil when the pool has
// no ExtraBlocks card). They sit in the cold buffer after the may, emblem and
// creature-counter records, so State itself does not grow (TestStateLayoutPinned).
func (s *State) xblkRecs() []xblkRec {
	l := s.cold.layout
	if l == nil || l.xblk == 0 {
		return nil
	}
	at := 0
	_ = coldView[mayPlay](nil, &at, l.may)
	_ = coldView[embRec](nil, &at, l.emb)
	_ = coldView[plusCreRec](nil, &at, l.ctrP)
	return coldView[xblkRec](s.cold.buf, &at, l.xblk)
}

// xblkOf is blocker b's second attacker (NoSlot none).
func (s *State) xblkOf(b Slot) Slot {
	if l := s.cold.layout; l == nil || l.xblk == 0 {
		return NoSlot
	}
	if a := s.xblkRecs()[b].a; a != 0 {
		return Slot(a - 1)
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
		return s.xblkRecs()[b].ord
	}
	return s.perms[b].bord
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
		add(b, p.blocking, p.bord)
		if x := s.xblkOf(b); x != NoSlot {
			add(b, x, s.xblkRecs()[b].ord)
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
		s.xblkRecs()[b] = xblkRec{}
	case p.blocking == a:
		p.blocking = NoSlot
		if x := s.xblkOf(b); x != NoSlot {
			p.blocking, p.bord = x, s.xblkRecs()[b].ord
			s.xblkRecs()[b] = xblkRec{}
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
		s.xblkRecs()[b] = xblkRec{}
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
// a two-attacker blocker ready. gorge offers the blocker's second block inside
// such a declaration too, and dirac would silently under-offer it.
const StallExtraBlockReq = 163

func init() { stallNames[StallExtraBlockReq] = "extrablockreq" }

// blockCap is how many attackers b may block (CR 509.1a): one, plus one for a
// printed CanBlockAmount$ 1 on itself unless it has lost its abilities.
func (s *State) blockCap(k *prep, b Slot) int {
	if k.gen != nil && k.gen.anyExtraBlock && k.t.Cards[s.cardAt(b)].ExtraBlocks != 0 && !s.genLost(k, b) {
		return 2
	}
	return 1
}

// blockerRoom: b may be declared against an attacker now: a ready blocker, or
// one that blocks a single attacker and has room for a second.
func (s *State) blockerRoom(k *prep, b Slot) bool {
	if s.blockerReady(k, b) {
		return true
	}
	p := &s.perms[b]
	return p.flags&pfBlocking != 0 && p.flags&pfTapped == 0 && s.isCreature(k, b) &&
		s.kw(k, b)&kop.KwCantBlock == 0 && s.blockCap(k, b) == 2 && s.xblkOf(b) == NoSlot
}

// extraBlockReady: some ready blocker of the defender may block two attackers.
func (s *State) extraBlockReady(k *prep) bool {
	for bm := s.ctrlM[s.blockDef()]; bm.any(); bm = bm.rest() {
		if b := bm.first(); s.blockerReady(k, b) && s.blockCap(k, b) == 2 {
			return true
		}
	}
	return false
}

// stallExtraBlockReq ends the game at StallExtraBlockReq, as optionsFull does.
//
//go:noinline
func (s *State) stallExtraBlockReq(d *Decision) bool {
	s.stall(StallExtraBlockReq)
	d.N = 0
	d.Kind = DkNone
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
			return &s.xblkRecs()[b].adiv
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
		n1 = int(s.xblkRecs()[b].div)
	} else {
		_, t := s.pt(k, a1)
		lethal := t - int(s.perms[a1].damage) - int(r.toPerm[a1])
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
	s.xblkRecs()[s.divA].div = amt[0]
	s.combatArm().divDone.set(s.divA)
	s.divA = NoSlot
}
