package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Reconfigure (CR 702.151; gorge engine/cards/kw_reconfigure.go): the keyword
// is two ordinary activated abilities of the card, lowered from gorge's
// expansion, and one rule that is not an ability:
//
//   - the attach half is a sorcery-speed Attach at "another target creature
//     you control" (the source is excluded from the pool, as gorge's target
//     ask excludes an attach SA's own source), the Equip shape;
//   - the unattach half is OpAttach with C&kop.AttachUnattach, offered only
//     while the source is attached to a creature (a lowered Present gate);
//   - CR 702.151b: an attached Reconfigure card is not a creature. That is the
//     type switch below, behind genPrep.anyReconfigure and keyed on the
//     attachment alone, the way gorge derives it (state.Object.
//     ReconfiguredAttached: AttachedTo != 0). The Creature card type and the
//     creature subtypes correlated with it (CR 205.1a) are stripped; the
//     non-creature subtypes (Equipment) stay. An Equipment riding an attached
//     Reconfigure card detaches by the ordinary CR 704.5n arm, because the
//     bearer is no creature any more.

// reconfigureAttached reports whether the permanent at sl is an attached
// Reconfigure card: attached, or its host left since the last state-based
// check (gorge keeps AttachedTo until the attachment SBA publishes the
// detach; lingerM).
func (s *State) reconfigureAttached(k *prep, sl Slot) bool {
	return k.t.Cards[s.cardAt(sl)].HasReconfigure && (s.perms[sl].attached != NoSlot || s.g.lingerM.has(sl))
}

// reconfigureTy is CR 702.151b's type switch over the derived types t of sl.
// A face-down permanent keeps its CR 708.2 set: the printed face, and the
// keyword with it, does not exist while it is face down (gorge
// reconfigureTypeSwitch).
func (s *State) reconfigureTy(k *prep, sl Slot, t kop.TypeMask) kop.TypeMask {
	if t&gtyFaceDown == 0 && s.reconfigureAttached(k, sl) {
		return t &^ kop.TCreature
	}
	return t
}

// reconfigureSubs is reconfigureTy over a derived subtype mask: an attached
// Reconfigure permanent keeps its non-creature subtypes (Equipment) and loses
// the creature subtypes that went with the Creature card type the switch took
// away (CR 205.1a, 702.151b). A card that is not attached, or is face down
// with its printed face absent (CR 708.2), keeps its mask and marker
// unchanged. The phasing projection (oracle/projchars.go) calls it too: the
// switch is derived live state, not a continuous effect, so it stands while
// the permanent is phased out.
func (s *State) reconfigureSubs(k *prep, sl Slot, subs kop.SubMask, all bool) (kop.SubMask, bool) {
	if s.faceDown(k, sl) || !s.reconfigureAttached(k, sl) {
		return subs, all
	}
	subs = subs.AndNot(k.gen.creatSub)
	return subs, false
}

// reconfigureSlots is the battlefield's attached Reconfigure permanents
// (refreshTy keeps their ovGenTy types).
//
//go:noinline
func (s *State) reconfigureSlots(k *prep) (m slotMask) {
	for o := s.occ; o.any(); o = o.rest() {
		if sl := o.first(); s.reconfigureAttached(k, sl) {
			m = m.with(sl)
		}
	}
	return m
}

// reconfigureRefresh re-derives the types once a Reconfigure card stopped
// being attached since its types were last derived; it reports whether one
// did.
//
//go:noinline
func (s *State) reconfigureRefresh(k *prep) bool {
	for m := s.g.tyM; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].ovr.flags&ovGenTy != 0 && s.perms[sl].gty&kop.TCreature == 0 && k.cards[s.cardAt(sl)].types&kop.TCreature != 0 &&
			k.t.Cards[s.cardAt(sl)].HasReconfigure && !s.reconfigureAttached(k, sl) {
			s.refreshTy(k)
			return true
		}
	}
	return false
}

// unattachSelf is the unattach half's resolution (CR 702.151a): the source
// stops being attached; an unattached source does nothing (gorge's stale
// answer refuses with a Note).
//
//go:noinline
func (s *State) unattachSelf(k *prep) {
	sl := s.objRef(k, kop.RefSelf)
	if sl == NoSlot || s.perms[sl].attached == NoSlot {
		return
	}
	p := &s.perms[sl]
	h := p.attached
	if s.occ.has(h) {
		s.touchSl(h)
	}
	p.attached = NoSlot
	s.touchSl(sl)
	if k.gen.anyAegis {
		s.aegisUnlink(k, sl, h, false) // zone_exilethenattach.go
	}
	s.refreshTy(k)
}
