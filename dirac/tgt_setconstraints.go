package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// target-SET constraints of a multi-object slot, one
// multi-select decision in gorge (decision.SetPropAdmits, rules/setprops.go,
// rules/target_modes.go totalCMCCappedCandidates):
//
//   - TgfSharedCrType: every object shares a creature type with the running
//     intersection (the first pick is free, but a mandatory 2-object slot's
//     first pick needs a partner: gorge offers completable picks only); a
//     changeling holds every creature type. The maximum is capped at the most
//     candidates holding one type (1 when none holds any);
//   - TgfDistinctNames: no two picks share a name (one CardDef per name); the
//     maximum is capped at the distinct names among the candidates;
//   - TgfMaxTotalMV: the picks' total mana value is at most TargetSpec.MaxMV
//     (graveyard cards only: their mana values never change, so there is no
//     resolution recheck).
//
// At resolution (CR 608.2b, rules/setprops.go narrowSetProps) a shared slot
// keeps, in recorded order, the legal targets that still share a type with the
// ones kept before them: xtSetNarrowed recomputes that from the current
// characteristics, so no State byte is needed.

// DebugTgtSetOff is the mutation switch: every target-set check admits.
var DebugTgtSetOff bool

// tgtSetMask is the TFlags bits of this file.
const tgtSetMask = kop.TgfSharedCrType | kop.TgfDistinctNames | kop.TgfMaxTotalMV

// NOTE: the battlefield shared-type path (crTypesOf's battlefield branch,
// xtSetNarrowed, anyTgtSetBf) has no lowered carrier in the committed pool:
// it was lockstepped only while Secret Tunnel was temporarily lowered (kgen
// now refuses it, exileOnMovedOne). Treat it as untested until a carrier
// (Raise the Draugr, Patch Up, ...) joins a lockstep pool.

// prepTgtSet sets anyTgtSetBf (some shared-creature-type slot is on the
// battlefield, where the creature types can change before resolution) and
// anyXBadMV (some slot has a total mana value cap).
func prepTgtSet(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			for ti := 0; ti < int(a.NTargets); ti++ {
				ts := &a.Targets[ti]
				if ts.TFlags&kop.TgfMaxTotalMV != 0 {
					g.anyXBadMV = true
				}
				if ts.TFlags&kop.TgfSharedCrType != 0 && ts.Kind == kop.TgtObject {
					g.anyTgtSetBf = true
				}
			}
		}
	}
}

// crTypesOf is the creature types of target object o: the matchup subtypes
// that are creature types, and whether it has every creature type (a
// changeling).
func (s *State) crTypesOf(k *prep, o Obj) (m kop.SubMask, all bool) {
	ob := &s.objs[o]
	var cre kop.SubMask
	if k.gen != nil {
		cre = k.gen.creatSub
	}
	if ob.zone == kop.ZoneBattlefield {
		for i := 0; i < len(k.t.Subtypes); i++ {
			if cre.Has(i) && s.hasSubGen(k, ob.slot, uint8(i)) {
				m = m.With(i)
			}
		}
		return m, s.hasSubGen(k, ob.slot, subOutside)
	}
	cd := &k.t.Cards[ob.card]
	for w := range m {
		m[w] = cd.Subtypes[w] & cre[w]
	}
	return m, cd.AllCreatureTypes
}

// sharesCr reports whether creature type sets a and b share a type: a side
// with every type shares with any non-empty set.
func (s *State) sharesCr(am kop.SubMask, aAll bool, bm kop.SubMask, bAll bool) bool {
	switch {
	case aAll:
		return bAll || bm.Any()
	case bAll:
		return am.Any()
	}
	return am[0]&bm[0]|am[1]&bm[1] != 0
}

// crAcc is the running intersection of the kept objects' creature types
// (SetPropAdmits / SetPropMerge); have is false before the first pick.
type crAcc struct {
	have bool
	m    kop.SubMask
	all  bool
}

func (a *crAcc) admits(s *State, m kop.SubMask, all bool) bool {
	if !a.have {
		return true
	}
	return s.sharesCr(a.m, a.all, m, all)
}

func (a *crAcc) merge(m kop.SubMask, all bool) {
	switch {
	case !a.have || a.all:
		a.m, a.all = m, all
	case !all:
		a.m[0] &= m[0]
		a.m[1] &= m[1]
	}
	a.have = true
}

// tgtSetCap is the most objects of cands the shared or distinct slot t can
// take together (gorge decision.SetPropCapacity).
func (s *State) tgtSetCap(k *prep, t *kop.TargetSpec, cands []uint8) uint8 {
	if DebugTgtSetOff {
		return 255
	}
	n := 0
	if t.TFlags&kop.TgfDistinctNames != 0 {
		var seen [MaxOptions]CardID
		for _, c := range cands {
			if TCode(c) == NoTarget || !isObjTarget(TCode(c)) {
				continue
			}
			id, dup := s.objs[c].card, false
			for i := 0; i < n; i++ {
				dup = dup || seen[i] == id
			}
			if !dup {
				seen[n] = id
				n++
			}
		}
		return uint8(min(n, 255))
	}
	if t.TFlags&kop.TgfSharedCrType == 0 {
		return 255
	}
	var cnt [kop.MaxSubtypes]int
	nAll, objs := 0, 0
	for _, c := range cands {
		if TCode(c) == NoTarget || !isObjTarget(TCode(c)) {
			continue
		}
		objs++
		m, all := s.crTypesOf(k, Obj(c))
		if all {
			nAll++
		}
		for i := range cnt {
			if m.Has(i) {
				cnt[i]++
			}
		}
	}
	best := nAll
	for _, v := range cnt {
		best = max(best, v+nAll)
	}
	if best == 0 && objs > 0 {
		best = 1
	}
	return uint8(min(best, 255))
}

// tgtSetBounds caps the maximum of a shared or distinct slot at its capacity
// (gorge setPropTargetBounds), as the slot is announced.
func (s *State) tgtSetBounds(k *prep, t *kop.TargetSpec, e *stackEntry, mt *multiTgt) {
	if t.TFlags&(kop.TgfSharedCrType|kop.TgfDistinctNames) == 0 {
		return
	}
	var buf [MaxOptions]uint8
	n := s.targetCands(k, t, e.ctrl, s.srcSlot(), e.obj, &buf)
	if c := s.tgtSetCap(k, t, buf[:n]); c < mt.max {
		mt.max = max8(c, mt.min)
	}
}

// tgtSetAdmit reports whether candidate c may join the picks of slot ti of the
// entry being built; cands is the slot's whole candidate list.
func (s *State) tgtSetAdmit(k *prep, t *kop.TargetSpec, e *stackEntry, ti uint8, c uint8, cands []uint8) bool {
	if DebugTgtSetOff || !isObjTarget(TCode(c)) {
		return true
	}
	picked := s.g.mt.picked
	x := s.xtOf(e)
	if t.TFlags&kop.TgfMaxTotalMV != 0 {
		tot := s.graveMV(k, Obj(c))
		if picked > 0 {
			tot += s.graveMV(k, Obj(e.tgt[ti]))
		}
		for i := 0; i < int(x.n); i++ {
			if x.slot[i] == ti {
				tot += s.graveMV(k, Obj(x.code[i]))
			}
		}
		if tot > int(t.MaxMV) {
			return false
		}
	}
	if t.TFlags&kop.TgfDistinctNames != 0 {
		id := s.objs[c].card
		if picked > 0 && s.objs[e.tgt[ti]].card == id {
			return false
		}
		for i := 0; i < int(x.n); i++ {
			if x.slot[i] == ti && s.objs[x.code[i]].card == id {
				return false
			}
		}
	}
	if t.TFlags&kop.TgfSharedCrType != 0 {
		cm, ca := s.crTypesOf(k, Obj(c))
		if picked == 0 {
			if t.Min < 2 {
				return true
			}
			for _, d := range cands {
				if d == c || TCode(d) == NoTarget || !isObjTarget(TCode(d)) {
					continue
				}
				if dm, da := s.crTypesOf(k, Obj(d)); s.sharesCr(cm, ca, dm, da) {
					return true
				}
			}
			return false
		}
		var acc crAcc
		pm, pa := s.crTypesOf(k, Obj(e.tgt[ti]))
		acc.merge(pm, pa)
		for i := 0; i < int(x.n); i++ {
			if x.slot[i] == ti {
				pm, pa = s.crTypesOf(k, Obj(x.code[i]))
				acc.merge(pm, pa)
			}
		}
		return acc.admits(s, cm, ca)
	}
	return true
}

// xtSetNarrowed reports whether extra target j of e survives the shared
// creature type narrowing at resolution (narrowSetProps): walk the slot's
// objects in recorded order, the primary first, skipping the ones no longer
// legal, and keep each that still shares a type with those kept before it.
func (s *State) xtSetNarrowed(k *prep, e *stackEntry, j int) bool {
	if DebugTgtSetOff {
		return true
	}
	x := s.xtOf(e)
	slot := x.slot[j]
	var acc crAcc
	for i := -1; i <= j; i++ {
		var c uint8
		if i < 0 {
			c = e.tgt[slot]
			if !s.targetStillLegal(k, e, int(slot)) {
				continue
			}
		} else {
			if x.slot[i] != slot {
				continue
			}
			c = x.code[i]
			if !s.xtLegalBase(k, e, i) {
				continue
			}
		}
		if !isObjTarget(TCode(c)) {
			continue
		}
		m, all := s.crTypesOf(k, Obj(c))
		if !acc.admits(s, m, all) {
			if i == j {
				return false
			}
			continue
		}
		acc.merge(m, all)
	}
	return true
}

// xtLegalRes is extra target j of e for the op execEachTarget runs: still
// legal now, except that a slot with a mana value cap (Scout for Survivors'
// "return up to three ... put a +1/+1 counter on each of them") keeps the
// legality the resolution began with (res.xbad): the objects the first op
// moved are new objects, which a live recheck would drop from the second op,
// while gorge binds the parent link's targets once (CR 608.2b).
func (s *State) xtLegalRes(k *prep, e *stackEntry, j int, slot uint8) bool {
	if j < 8 {
		return s.res.xbad&(1<<j) == 0 // judged as the resolution started (CR 608.2b): an earlier op may have moved it (Pull's Pump)
	}
	return s.xtLegal(k, e, j)
}

// biteEachTarget is a bite whose damage target slot holds several objects (Betrayal
// at the Vault's "each of two other target creatures"): the source deals its power to
// each still-legal one, in one damage batch (CR 120.3, 608.2b). It reports whether in
// is such a bite.
func (s *State) biteEachTarget(k *prep, in *kop.Instr, a Slot) bool {
	e := &s.res.e
	if k.gen == nil || s.xtOf(e).n == 0 {
		return false
	}
	sl, ok := tgtSlot(kop.Ref(in.C))
	if !ok || !s.slotHasExtra(e, uint8(sl)) {
		return false
	}
	if a == NoSlot || !s.isCreature(k, a) {
		return true
	}
	pa := s.power(k, a)
	var hits [maxXTgt + 1]dmgHit
	n := 0
	hit := func() {
		if b := s.objRef(k, kop.Ref(in.C)); b != NoSlot && (s.isCreature(k, b) || s.typesAt(k, b)&kop.TPlaneswalker != 0) {
			hits[n] = dmgHit{src: a, dst: b, n: int16(min(max(pa, -1<<14), 1<<14))}
			n++
		}
	}
	saveT, saveV, saveBad := e.tgt[sl], e.tver[sl], s.res.bad
	hit()
	for j := 0; j < int(s.xtOf(e).n) && n < len(hits); j++ {
		if s.xtOf(e).slot[j] != uint8(sl) || !s.xtLegalRes(k, e, j, uint8(sl)) {
			continue
		}
		e.tgt[sl], e.tver[sl] = s.xtOf(e).code[j], s.xtOf(e).ver[j]
		s.res.bad &^= 1 << sl
		hit()
	}
	e.tgt[sl], e.tver[sl], s.res.bad = saveT, saveV, saveBad
	s.dealHits(k, hits[:n])
	return true
}
