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
// Its other reach is a gains-of Equip grant (prep.gainsOfAttach,
// prepGainsOfAttach below): a permanent that is not an attachment gains an
// activated Equip ability (Marvin, Murderous Mimic) and attaches itself. The
// creature arm unattaches it; a noncreature recipient is left attached (gorge
// attachedKindIllegal's grantedAttachKeyword carve-out, grantedEquipKeyword
// below), and a printed attachment face whose derived types lost the type
// still detaches.
//
// Not modelled (no lowered carrier; crcoverage 704.5p records them SILENT): an
// attached battle. The attached-Aura reach is modelled under genPrep.equipAnim (set by prepAuraAnim)
// (static_equipanimated.go): a Creature-adding override whose filter can reach
// an attached Aura — Mercurial Transformation's Octopus animate, which removes
// the Aura's card types, leaves the permanent a plain creature and gorge's
// creature arm unattaches it (pair 29 of tail-46); an animate that keeps the
// Aura type takes the same arm and then the next pass's CR 704.5m graveyard.
// 0 B: the link is the permanent's attached slot.

// DebugAttachedCreatureStays leaves an attached creature attached (mutation
// testing only: the lockstep must catch it).
var DebugAttachedCreatureStays bool

// sbaAttachedCreatures runs the arm over the battlefield once sba's
// per-permanent loop is done, as gorge's attachmentSBAs runs after the pass's
// casualties: a permanent or bearer that leaves in this pass (all) is left to
// the bearer-left handling. It reports whether an Equipment became unattached
// (gorge's changed pass). Reached only under genPrep.equipAnim (set by
// prepEquipAnim or prepAuraAnim) or prep.gainsOfAttach, so the hot
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
// runs first. A noncreature, non-Equipment recipient of a live gains-of Equip
// grant stays attached: gorge attachedKindIllegal's grantedAttachKeyword is the
// one route that attaches a permanent that is not itself an attachment type
// (CR 704.5p's second arm leaves it), and only a printed attachment face whose
// derived types lost the type detaches despite it.
func (s *State) sbaAttachedCreature(k *prep, sl Slot, t kop.TypeMask) bool {
	if DebugAttachedCreatureStays || s.perms[sl].attached == NoSlot {
		return false
	}
	if k.gen != nil && k.gen.anyBestow && k.t.Cards[s.cardAt(sl)].HasBestow {
		return false
	}
	if k.gen != nil && k.gen.anyReconfigure && s.reconfigureAttached(k, sl) {
		return false // CR 702.151b: an attached Reconfigure card is no creature (kw_reconfigure.go)
	}
	if t&kop.TCreature != 0 {
		// CR 704.5p's creature arm: any attached creature detaches and stays,
		// an Aura among its types included (gorge's attachedKindIllegal reads
		// the creature arm before its Aura arms; the next pass's "Aura
		// attached to nothing" then takes CR 704.5m).
	} else if t&kop.TEquipment != 0 {
		return false // a noncreature Equipment stays (gorge's arm 2 skips it)
	} else if t&kop.TAura != 0 {
		return false // a still-Aura noncreature: the Aura arms in sbaScan govern it
	} else if k.t.Cards[s.cardAt(sl)].Types&(kop.TEquipment|kop.TAura) != 0 {
		// gorge printsAttachmentType: the printed face is an attachment
		// type whose derived types lost it (a copy of a plain artifact),
		// so the grant carve-out does not apply and the attachment is
		// illegal (CR 704.5p arm 2, CR 301.5c).
	} else if k.gainsOfAttach && s.grantedEquipKeyword(k, sl) {
		return false // gorge's grantedAttachKeyword carve-out (grantedEquipKeyword)
	}
	// Anything else reaching here (a noncreature permanent that is no
	// attachment type and holds no live Equip grant) is gorge's "non-attachment
	// permanent becomes unattached" arm, so it detaches below.
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

// grantedEquipKeyword reports whether sl is a live recipient of a
// GainsAbilitiesOf$ static that grants it an Equip ability right now: some
// battlefield permanent the static's GainsOf filter matches carries a modelled
// activated Equip ability (static_gainsof.go's walk grants it, the recipient
// activates it and OpAttach attaches the recipient itself). This mirrors gorge
// attachedKindIllegal's grantedAttachKeyword (derivedKeywordParam Equip), whose
// carve-out keeps such a non-attachment attached; only the gains-of grant is
// lowered (kop has no Fortify flag), so the check runs only under
// prep.gainsOfAttach. Cold, and read only from sbaAttachedCreature.
func (s *State) grantedEquipKeyword(k *prep, sl Slot) bool {
	if s.genLost(k, sl) {
		return false // a recipient that lost all abilities gains nothing (static_gainsof.go)
	}
	var order [MaxPerm]Slot
	n := s.listenerOrder(s.statM, &order)
	for oi := 0; oi < n; oi++ {
		src := order[oi]
		if s.genLost(k, src) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.GainsOf == 0 {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: s.perms[src].ctrl, obj: s.perms[src].obj}
			if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
				continue // sl is not the static's recipient
			}
			var ford [MaxPerm]Slot
			fn := s.listenerOrder(s.occ, &ford)
			for fi := 0; fi < fn; fi++ {
				fs := ford[fi]
				if s.faceDown(k, fs) || !s.matchPermGen(k, a.Static.GainsOf, fs, st.ctrl, st.sl, layerKw) {
					continue // a face-down source grants nothing (CR 708.2a)
				}
				fcd := &k.t.Cards[s.cardAt(fs)]
				for j := range fcd.Abilities {
					if gainedModelled(&fcd.Abilities[j], j) && fcd.Abilities[j].AFlags&kop.AfEquip != 0 {
						return true
					}
				}
			}
		}
	}
	return false
}

// prepGainsOfAttach sets prep.gainsOfAttach: the CR 704.5p arm is reachable
// also when a permanent that is not itself an attachment type can gain an Equip
// ability — a GainsAbilitiesOf$ static (kop.Static.GainsOf) granting an
// activated AfEquip ability lets its recipient attach itself (Marvin, Murderous
// Mimic activating a manifested Equipment's Equip). The arm then unattaches the
// recipient when it is a creature; a noncreature recipient is left attached
// (sbaAttachedCreature's granted-keyword carve-out).
func prepGainsOfAttach(t *kop.Table, p *prep) {
	gainsOf, equip := false, false
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.GainsOf != 0 {
				gainsOf = true
			}
			if a.AFlags&kop.AfEquip != 0 {
				equip = true
			}
			if gainsOf && equip {
				p.gainsOfAttach = true
				return
			}
		}
	}
}
