package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Bestow (CR 702.103; gorge rules/bestow.go): the hand offers a
// card printed with K:Bestow a second cast, AkBestow (gorge's "bestowed"
// cast option), for its bestow cost (CardDef.BestowCost). The spell is an
// Aura with enchant creature (CardDef.BestowTgt) and resolves attached to
// its target; if the target is illegal as it begins resolving, it ceases
// to be bestowed and resolves as a creature spell (CR 702.103e). On the
// battlefield the card is an Aura and not a creature while it is attached
// (gorge state.Object.BestowedAttached: attached and printed with bestow,
// whatever way it got attached), and when its host is gone, is not a
// creature or is protected from it, it detaches and stays, a creature
// again (CR 702.103f; gorge attachmentSBAs' bestowed arm).
//
// The extension, all behind genState.anyBestow:
//   - the cast is its own action kind (AkBestow, as AkSneak / AkCastCasualty):
//     every AkCast mode bit 0x01..0x80 is taken, and a separate kind keeps
//     the plain cast's mode byte untouched. Its build is a plain hand cast
//     (beginCast with castFace, which keeps it out of the plain cast's
//     lowering) whose stack entry carries modeBestowed;
//   - stackEntry.mode bit 3 (modeBestowed) on a permanent spell (ab 255,
//     not face down): the modal bits 0..3 are a spell ability's, and a
//     permanent spell has none. buildCost0 prices BestowCost for it and
//     tspecE returns BestowTgt as its slot 0;
//   - from payment on (gorge's pay-time FlagBestowed) the spell's object
//     carries slot slotBestowSpell, as a face-down spell carries
//     slotFaceDownSpell, so a card filter reads it as an Aura and not a
//     creature without a stack walk (matchCardOne: SpellCast triggers,
//     "target creature spell");
//   - typesGen switches Creature to Aura for an attached bestow permanent
//     (bestowTy), and refreshTy keeps those slots on ovGenTy, so typesAt
//     (combat, SBA, filters) reads the switch;
//   - the SBA's Aura arm detaches a bestowed Aura instead of putting it into
//     the graveyard (sbaBestowDetach).

// modeBestowed: stackEntry.mode on a bestowed permanent spell.
const modeBestowed = 1 << 3

// slotBestowSpell is object.slot of a bestowed spell on the stack after
// payment (a stack object's slot is otherwise NoSlot or slotFaceDownSpell).
// placeFrom, unpushSpell and enterNF replace it.
const slotBestowSpell Slot = NoSlot - 2

// bestowedEntry reports whether e is a bestowed permanent spell.
func (e *stackEntry) bestowedEntry() bool {
	return e.ab == 255 && e.kind == seSpell && e.mode&(seFaceDown|modeBestowed) == modeBestowed
}

// tspecE is tspec for stack entry e: a bestowed spell's one target is its
// enchant creature (BestowTgt).
func tspecE(k *prep, e *stackEntry, i int) *kop.TargetSpec {
	if e.mode&(seFaceDown|modeBestowed) == modeBestowed && e.ab == 255 && e.kind == seSpell {
		if i == 0 {
			return &k.t.Cards[e.card].BestowTgt
		}
		return nil
	}
	if e.kind == seSpell && e.ab < 255 {
		if a := &k.t.Cards[e.card].Abilities[e.ab]; i >= int(a.NTargets) && a.Repeats() {
			// an extra instance of a repeated mode takes its mode's spec (kop/repeat.go)
			if t, ok := a.SlotOwner(e.mode, i); ok {
				return &a.Targets[t]
			}
			return nil
		}
	}
	return tspec(k, e.card, e.ab, i)
}

// offerBestowHand offers hand card o's bestowed cast (gorge
// legal_walk_hand.go: offerCastable at the bestow cost and
// castTargetsAvailable over the synthesized enchant creature SA). The
// caller has checked the card's timing, its prohibition and the plain
// cast's gates.
//
//go:noinline
func (s *State) offerBestowHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if DebugNoBestow {
		return
	}
	cd := &k.t.Cards[c]
	bc := s.adjustCost(k, p, c, o, cd.BestowCost) // the reduction applies to an alternative cost too (CR 601.2f)
	if len(k.gen.grantPC) != 0 && s.g.availX&availWalk != 0 {
		// gorge's potential walk counts a restricted mana ability only for a
		// payment its restriction admits (potentialRestrictionAdmits): a
		// hand-barred grant puts no cast on the walk grantCap reads
		var ha avail
		s.walkHandAvail(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}, &ha)
		av = &ha
	}
	if !av.affords(&bc, pipMaskOf(&bc), 0) && (!s.rbAny(p) && !k.restrSrc || !s.restrFallback(k, p, c, s.restrOriginOfObj(o), &bc, av)) {
		return
	}
	if s.castHeldOut(o) {
		return
	}
	var buf [MaxOptions]TCode
	if k.gen != nil && s.g.probe.kind == 0 {
		// the spell's colours decide what it may enchant: a monocolored Hydra cannot
		// enchant a creature with protection from monocolored (CR 702.16b)
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
		defer func() { s.g.probe = probe{} }()
	}
	if s.targetCands(k, &cd.BestowTgt, p, NoSlot, o, &buf) == 0 {
		return
	}
	d.add(MakeAction(AkBestow, TCode(o), 0, 0))
}

// beginBestow casts hand card o bestowed. gorge prices the bestowed cast as
// a potential play (altBestow potential) unless the floating pool alone pays
// it, so the oracle lowers it then: the witness's sources (the largest X
// they pay) are tapped first. A cast the pool pays is an ordinary option.
//
//go:noinline
func (s *State) beginBestow(k *prep, o Obj) {
	s.beginCast(k, o, castFace, 0)
	s.bld().e.mode |= modeBestowed
	cost := s.buildCost(k)
	// gorge's planner prices the granted delve's credit (offerPlay's choke
	// point), so the pool gate prices the credited cost too; a twobrid cost
	// is leaf-credited by the gate's avail (gateDelveCredit)
	cost = s.gateDelveCredit(k, s.prio, s.bld().e.card, o, false, cost)
	if s.sneakPoolPays(k, s.prio, s.bld().e.card, o, &cost) {
		return
	}
	s.bld().bfl |= bflLowered
	s.lowerWitness(k, s.prio)
}

// bestowPaid marks a bestowed spell's object once its costs are paid
// (gorge's pay-time FlagBestowed): from then on a card filter reads it as
// an Aura spell, not a creature spell (CR 702.103b).
func (s *State) bestowPaid(e *stackEntry) {
	if ob := &s.objs[e.obj]; ob.zone == kop.ZoneStack {
		s.touch(e.obj)
		ob.slot = slotBestowSpell
	}
}

// bestowSpellObj reports whether o is a bestowed spell on the stack.
func (s *State) bestowSpellObj(o Obj) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneStack && ob.slot == slotBestowSpell
}

// bestowAttached reports whether the permanent at sl is an attached bestow
// card: attached, or its host left since the last state-based check (gorge
// keeps AttachedTo until the attachment SBA publishes the detach; lingerM).
func (s *State) bestowAttached(k *prep, sl Slot) bool {
	return k.t.Cards[s.cardAt(sl)].HasBestow && (s.perms[sl].attached != NoSlot || s.g.lingerM.has(sl))
}

// bestowTy is CR 702.103b's type switch over the derived types t of sl.
func (s *State) bestowTy(k *prep, sl Slot, t kop.TypeMask) kop.TypeMask {
	if s.bestowAttached(k, sl) {
		return t&^kop.TCreature | kop.TAura
	}
	return t
}

// bestowSlots is the battlefield's attached bestow permanents (refreshTy
// keeps their ovGenTy types).
//
//go:noinline
func (s *State) bestowSlots(k *prep) (m slotMask) {
	for o := s.occ; o.any(); o = o.rest() {
		if sl := o.first(); s.bestowAttached(k, sl) {
			m = m.with(sl)
		}
	}
	return m
}

// bestowResolve begins a bestowed spell's resolution (CR 702.103e, gorge
// stack.go before the fizzle check): an illegal target ends the bestow, and
// the spell resolves as a creature spell with no target.
//
//go:noinline
func (s *State) bestowResolve(k *prep, e *stackEntry) {
	if s.targetStillLegal(k, e, 0) {
		return
	}
	e.mode &^= modeBestowed
	e.nt = 0
	e.tgt[0] = TCode(NoTarget)
	s.res.e = *e
	s.stack[s.sp-1].mode, s.stack[s.sp-1].nt, s.stack[s.sp-1].tgt[0] = e.mode, 0, TCode(NoTarget)
	if ob := &s.objs[e.obj]; ob.slot == slotBestowSpell {
		s.touch(e.obj)
		ob.slot = NoSlot
	}
}

// enterAttached puts an Aura spell e onto the battlefield attached to its
// target. The enters check sees the attachment (CR 603.6a; gorge
// moveResolvedOffStack enters it attached): an Aura that removes abilities
// silences the host's own "whenever an enchantment enters".
//
//go:noinline
func (s *State) enterAttached(k *prep, e *stackEntry) Slot {
	sl := s.enterNF(k, e.obj, e.ctrl, false, kop.ZoneStack, 0)
	if isPlayerTarget(e.tgt[0]) {
		// an Enchant:Player spell (the curses): the target is the player
		// code, not an object (gorge auraEntry's toPlayer settle, the
		// "attach to player" fold)
		s.attachPlayer(k, sl, CodeSeat(e.tgt[0]))
	} else {
		s.attachTo(k, sl, s.objs[e.tgt[0]].slot)
	}
	s.touchSl(sl)
	if k.gen != nil {
		s.refreshTy(k)
	}
	if s.anyETB(k) {
		s.fireObj(k, kop.TrigETB, sl, e.obj, e.ctrl, 0)
	}
	return sl
}

// sbaBestowDetach is the attachment SBA's bestowed arm (CR 702.103f, gorge
// attachmentSBAs) for the Aura at sl, unattached or illegally attached: a
// bestow card detaches and stays, a creature again (true); any other Aura
// is the caller's (false).
//
//go:noinline
func (s *State) sbaBestowDetach(k *prep, sl Slot) bool {
	if k.gen == nil || !k.gen.anyBestow || !k.t.Cards[s.cardAt(sl)].HasBestow {
		return false
	}
	p := &s.perms[sl]
	if p.attached != NoSlot {
		s.touchSl(p.attached)
	}
	p.attached = NoSlot
	s.g.lingerM.clr(sl)
	s.touchSl(sl)
	return true
}

// bestowRefresh re-derives the types once a bestow card stopped being
// attached since its types were last derived; it reports whether one did.
//
//go:noinline
func (s *State) bestowRefresh(k *prep) bool {
	for m := s.g.tyM; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].ovr.flags&ovGenTy != 0 && s.perms[sl].gty&kop.TAura != 0 && k.t.Cards[s.cardAt(sl)].HasBestow && !s.bestowAttached(k, sl) {
			s.refreshTy(k)
			return true
		}
	}
	return false
}

// DebugNoBestow withholds the bestowed cast (the bestow mutation test
// proves the pool casts it).
var DebugNoBestow bool
