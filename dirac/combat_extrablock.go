package dirac

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
// attackers, meaningful only while combatArm().divDone.has(blocker).
type xblkRec struct{ a, ord, div uint8 }

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
	_ = coldView[uint8](nil, &at, l.ctrP)
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
