package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Umbra armor (CR 702.89a, kop.KwUmbraArmor): "If enchanted permanent would
// be destroyed, instead remove all damage marked on it and destroy this
// Aura." gorge (engine/effects ReplaceUmbraArmor) consults the bearer's
// attached Auras at every destruction choke point after the R:Event$ Destroy
// lines, the regeneration shield and a shield counter: a destroy effect, a
// destroy-all, the lethal-damage sweep (a deathtouch mark included). It is not
// regeneration, so a can't-be-regenerated destruction does not stop it; it is
// one-shot (the Aura is the charge) and does not tap the bearer or remove it
// from combat. Which Aura answers is the first one attached with the keyword in
// the deterministic scan gorge's UmbraArmorAura takes (seat order, then the
// battlefield list, which is the entry timestamp here).
//
// Only the printed keyword is read: a granted umbra armor (Umbra Mystic) stays
// refused in kgen.

// kwUmbraArmor is the manifest keyword CardDef.UmbraArmor stands for.
const kwUmbraArmor = kop.KwUmbraArmor

// prepUmbra sets genPrep.anyUmbra when a card is an umbra Aura.
func prepUmbra(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if t.Cards[i].UmbraArmor {
			g.anyUmbra = true
		}
	}
}

// umbraAura is the Aura on the battlefield attached to host that carries
// umbra armor, or NoSlot: the first by seat, then battlefield order. An Aura
// in skip is already gone (a destroy-all's earlier victim) and does not
// answer; one that lost its abilities does not carry the keyword.
func (s *State) umbraAura(k *prep, host Slot, skip slotMask) Slot {
	if k.gen == nil || !k.gen.anyUmbra {
		return NoSlot
	}
	var cand slotMask
	for m := s.occ.andNot(skip); m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].attached == host && k.t.Cards[s.cardAt(sl)].UmbraArmor && !s.genLost(k, sl) {
			cand = cand.with(sl)
		}
	}
	if !cand.any() {
		return NoSlot
	}
	var ord [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < MaxSeats; p++ {
		n = s.battlefieldOrder(p, cand.and(s.ctrlM[p]), ord[:], n)
	}
	if n == 0 {
		return NoSlot
	}
	return ord[0]
}

// umbraHeal is the replacement's first half: all damage and the deathtouch
// mark leave host (gorge removes the Damage and the "Deathtouched" counter,
// which stays listed at 0).
func (s *State) umbraHeal(host Slot) {
	p := &s.perms[host]
	s.touchSl(host)
	if p.flags&pfDeathtouched != 0 {
		p.cseen |= cseenDT
	}
	p.damage = 0
	p.flags &^= pfDeathtouched | pfDtMore
}

// umbraSpare applies umbra armor to the destruction of the permanent at host:
// true when an Aura answered, the damage gone and the Aura destroyed
// instead (destroySpared, after the shield counter).
//
//go:noinline
func (s *State) umbraSpare(k *prep, host Slot) bool {
	aura := s.umbraAura(k, host, slotMask{})
	if aura == NoSlot {
		return false
	}
	s.umbraHeal(host)
	s.leaveParked(k, aura)
	return true
}

// umbraBatch is umbraSpare for the state-based sweep: the Aura is returned,
// not moved, so the pass's batch destroys it with the pass's other
// casualties (the caller adds it to its dying set). NoSlot when none answers.
//
//go:noinline
func (s *State) umbraBatch(k *prep, host Slot) Slot {
	aura := s.umbraAura(k, host, slotMask{})
	if aura != NoSlot {
		s.umbraHeal(host)
	}
	return aura
}

// umbraAll is OpDestroyAll's umbra pass (after shieldAll): each victim of hit
// that an Aura answers leaves hit and the Aura joins it. Victims run in
// battlefield order and an Aura that is itself an earlier victim is already
// gone when its bearer's turn comes (gorge destroys one victim at a time and
// skips the ones that left).
//
//go:noinline
func (s *State) umbraAll(k *prep, hit slotMask) slotMask {
	if k.gen == nil || !k.gen.anyUmbra {
		return hit
	}
	var ord [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < MaxSeats; p++ {
		n = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), ord[:], n)
	}
	out := hit
	var before, gone slotMask // victims already visited; Auras consumed
	for _, sl := range ord[:n] {
		if gone.has(sl) {
			continue
		}
		aura := s.umbraAura(k, sl, before.and(hit))
		before = before.with(sl)
		if aura == NoSlot {
			continue
		}
		s.umbraHeal(sl)
		out = out.without(sl).with(aura)
		gone = gone.with(aura)
	}
	return out
}

// sbaSpare offers a lethal-damage casualty at sl to the shield counter and
// then to umbra armor (gorge replaceSBADestruction's order), true when one
// took the destruction. A shield counter leaves the damage marked, so the
// pass runs once more; an umbra Aura joins die, to be destroyed with the
// pass's other casualties.
func (s *State) sbaSpare(k *prep, sl Slot, die *slotMask) bool {
	if k.gen != nil && k.gen.anyRepl&rbShield != 0 && s.shieldDestroy(sl) {
		s.cfgx |= cfgxShieldPass // CR 122.1c: the counter goes instead, the damage stays marked for the next pass
		return true
	}
	if aura := s.umbraBatch(k, sl); aura != NoSlot {
		*die = die.with(aura) // CR 702.89a: the damage left with the keyword's use
		return true
	}
	return false
}

// shieldUmbraAll is OpDestroyAll's pass after regeneration: each victim holding
// a shield counter loses one instead, then each an umbra Aura answers keeps
// living and the Aura is destroyed instead.
func (s *State) shieldUmbraAll(k *prep, hit slotMask) slotMask {
	if k.gen != nil && k.gen.anyRepl&rbShield != 0 {
		hit = s.shieldAll(hit) // CR 122.1c
	}
	return s.umbraAll(k, hit)
}
