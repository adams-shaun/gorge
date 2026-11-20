package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Can block any number of creatures" (CanBlockAny$ True, CR 509.1a): a
// blocker with kop.ExtraBlocksAny (printed, CardDef.ExtraBlocks) or a
// StatExtraBlock grant whose Static.A is kop.ExtraBlocksAny (Entangler) is
// offered every attacker it can block, one declared pair at a time as gorge's
// multi-pick does (BlockPairAdmits, with the blocker's GroupLimits entry
// unbounded), and its controller divides its combat damage among all the
// attackers it blocks (CR 510.1d; combat_extrablock.go blockerMultiDamage).
//
// Every attacker after the first is a record of the cold table xblkRecs,
// which a pool with a CanBlockAny$ source sizes maxXblkAny. A blocker that
// would need a record when the table is full stops the kernel at
// StallExtraBlockCap, a declared capacity limit: the table is sized so that
// the pods' go-wide games do not reach it.
//
// A requirement-maximising declaration (a MustBlock duty or a lure) with such
// a blocker in play is not modelled: its quota search (combat_mustbeblocked.go)
// holds a blocker to at most two pairs. It stops at the same stall.

// maxXblkAny is how many further blocks one combat holds records for in a
// pool with a CanBlockAny$ source.
const maxXblkAny = 64

// blockCapAny is blockCap's answer for a blocker without a limit.
const blockCapAny = 255

// useBlockAny: a table with a CanBlockAny$ source sizes the larger
// further-block table (coldXblk) and reads the unbounded capacity.
var useBlockAny = newGenUse(tableHasBlockAny)

func tableHasBlockAny(t *kop.Table) bool {
	for i := range t.Cards {
		c := &t.Cards[i]
		if c.ExtraBlocks == kop.ExtraBlocksAny {
			return true
		}
		for j := range c.Abilities {
			if isBlockAnyGrant(&c.Abilities[j]) {
				return true
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if isBlockAnyGrant(&t.Emblems[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// isBlockAnyGrant: a is a StatExtraBlock of amount any.
func isBlockAnyGrant(a *kop.Ability) bool {
	return a.Kind == kop.AbStatic && a.Static.Kind == kop.StatExtraBlock && a.Static.A == int16(kop.ExtraBlocksAny)
}

// blockAny: b can block any number of creatures, by a printed CanBlockAny$
// (unless it has lost its abilities) or a grant that reaches it. It
// dominates every CanBlockAmount$ source, as gorge's BlockCapacity returns at
// the first CanBlockAny$ one.
func (s *State) blockAny(k *prep, b Slot) bool {
	if k.gen == nil || !k.gen.uses.has(useBlockAny) {
		return false
	}
	if k.t.Cards[s.cardAt(b)].ExtraBlocks == kop.ExtraBlocksAny && !s.genLost(k, b) {
		return true
	}
	return s.blockAnyGrant(k, b)
}

// blockAnyGrant: a StatExtraBlock of amount any whose filter matches b.
//
//go:noinline
func (s *State) blockAnyGrant(k *prep, b Slot) bool {
	found := false
	s.eachStatic(k, func(st statSrc) bool {
		if a := st.a; isBlockAnyGrant(a) && s.statCond(k, st) &&
			s.matchPermGen(k, a.Static.Filter, b, st.ctrl, st.sl, layerFull) {
			found = true
			return false
		}
		return true
	})
	return found
}

// blockAnyRoom: blocker b of a CanBlockAny$ source, already blocking, may be
// declared against another attacker: a record is free. With the table full
// and an attacker still blockable it stops the kernel (StallExtraBlockCap).
func (s *State) blockAnyRoom(k *prep, b Slot) bool {
	if s.xblkRoom() {
		return true
	}
	if s.thirdBlock(k, b) {
		s.stall(StallExtraBlockCap) // the table holds maxXblkAny further blocks
	}
	return false
}

// blockAnyInPlay: a defending creature that can block any number of creatures
// is ready or already blocking.
func (s *State) blockAnyInPlay(k *prep) bool {
	if k.gen == nil || !k.gen.uses.has(useBlockAny) {
		return false
	}
	for bm := s.ctrlM[s.blockDef()]; bm.any(); bm = bm.rest() {
		if b := bm.first(); s.blockAny(k, b) && (s.blockerRoom(k, b) || s.perms[b].flags&pfBlocking != 0) {
			return true
		}
	}
	return false
}

// blockAnyReqStall ends the declaration at StallExtraBlockCap: the
// requirement search holds a blocker to two pairs.
func (s *State) blockAnyReqStall(d *Decision) bool {
	s.stall(StallExtraBlockCap)
	d.N = 0
	d.Kind = DkNone
	return true
}
