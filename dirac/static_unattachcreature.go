package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 704.5p. gorge attachmentSBAs
// attachedKindIllegal unattaches an attached object that is a creature (derived
// types) once the bearer-left and bestowed arms have passed it; the permanent
// stays on the battlefield. The Equipment half is modelled here: an Equipment
// that an animator made a creature (Ensouled Scimitar, Relic's Roar, Karn's
// Touch, Ensoul Artifact, ...) leaves its bearer and does not reattach when
// the animation ends. sba runs it after its per-permanent loop; an Equipment
// creature whose bearer stopped being a creature in the same pass is
// unattached by the loop's CR 704.5n arm instead (same result).
//
// Its other half is modelled too: an attachment whose derived types lost Equipment,
// by becoming a copy of another artifact (Shuri, Wakandan Inventor).
//
// Not modelled (no lowered carrier; crcoverage 704.5p records them SILENT): an
// attached Aura that becomes a creature (it would take the "Aura attached to
// nothing" graveyard arm on gorge's next pass) and an attached battle.
// 0 B: the link is the permanent's attached slot.

// DebugAttachedCreatureStays leaves an attached creature attached (mutation
// testing only: the lockstep must catch it).
var DebugAttachedCreatureStays bool

// sbaAttachedCreatures runs the arm over the battlefield once sba's
// per-permanent loop is done, as gorge's attachmentSBAs runs after the pass's
// casualties: a permanent or bearer that leaves in this pass (all) is left to
// the bearer-left handling. It reports whether an Equipment became unattached
// (gorge's changed pass). Reached only under genPrep.equipAnim, so the hot
// per-permanent loop carries none of it.
//
//go:noinline
func (s *State) sbaAttachedCreatures(k *prep, all slotMask) bool {
	un := false
	for m := s.occ.andNot(all); m.any(); m = m.rest() {
		sl := m.first()
		if h := s.perms[sl].attached; h != NoSlot && !all.has(h) && s.sbaAttachedCreature(k, sl, s.typesAt(k, sl)) {
			un = true
		}
	}
	return un
}

// sbaAttachedCreature unattaches permanent sl (types t) when it is an attached
// creature or no longer an Equipment, and reports whether it did. An Aura is left
// to the Aura arm and a bestow card to the bestowed arm (CR 702.103f), which gorge
// runs first.
func (s *State) sbaAttachedCreature(k *prep, sl Slot, t kop.TypeMask) bool {
	if DebugAttachedCreatureStays || t&kop.TCreature == 0 && t&kop.TEquipment != 0 || t&kop.TAura != 0 || s.perms[sl].attached == NoSlot {
		return false
	}
	if k.gen != nil && k.gen.anyBestow && k.t.Cards[s.cardAt(sl)].HasBestow {
		return false
	}
	p := &s.perms[sl]
	h := p.attached
	if s.occ.has(h) {
		s.touchSl(h)
	}
	if k.gen != nil && k.gen.anyUnattach {
		// gorge attachedKindIllegal's Unattached (CR 704.5p, attachmentSBAs):
		// an attached creature or non-attachment detaches and stays, the
		// publish carrying the bearer
		s.fireUnattached(k, sl, h)
	}
	p.attached = NoSlot
	s.touchSl(sl)
	if k.gen != nil && k.gen.anyAegis {
		s.aegisUnlink(k, sl, h, false) // zone_exilethenattach.go
	}
	return true
}
