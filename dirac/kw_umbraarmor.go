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
// in skip has already answered for another victim and does not answer again;
// one that lost its abilities does not carry the keyword.
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
	return s.umbraSpareIn(k, host, slotMask{})
}

// umbraSpareIn is umbraSpare for a batch of simultaneous departures: when the
// Aura is itself one of the victims (CR 702.89a; a spell destroying the Aura
// and the creature it enchants at the same time), the save still applies but
// the Aura's own departure is the batch's, so it is not left here.
func (s *State) umbraSpareIn(k *prep, host Slot, batch slotMask) bool {
	aura := s.umbraAura(k, host, slotMask{})
	if aura == NoSlot {
		return false
	}
	s.umbraHeal(host)
	if !batch.has(aura) {
		s.leaveParked(k, aura)
	}
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
// that an Aura answers leaves hit and the Aura joins it. The answers are read
// from the pre-batch board, as gorge effDestroyAll's pre-read does (CR
// 702.89a): an Aura that is itself a victim still answers when its bearer's
// turn comes, whichever of the two the seat/battlefield order visits first.
// Only an Aura already consumed as an answer is skipped, so it saves at most
// one bearer.
//
//go:noinline
func (s *State) umbraAll(k *prep, hit slotMask) slotMask {
	return s.umbraAllIn(k, hit, nil)
}

// umbraAllIn is umbraAll; u records each host -> Aura answer so the caller's
// departure walk can put the Aura at its host's position (gorge's
// effDestroyAll destroys it while it processes that victim).
func (s *State) umbraAllIn(k *prep, hit slotMask, u *umbraOrder) slotMask {
	if k.gen == nil || !k.gen.anyUmbra {
		return hit
	}
	var ord [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < MaxSeats; p++ {
		n = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), ord[:], n)
	}
	out := hit
	var gone slotMask // Auras already consumed as an answer
	for _, sl := range ord[:n] {
		if gone.has(sl) {
			continue
		}
		aura := s.umbraAura(k, sl, gone)
		if aura == NoSlot {
			continue
		}
		s.umbraHeal(sl)
		if u != nil {
			u.add(sl, aura)
			if hit.has(aura) {
				u.victim = u.victim.with(aura) // destroyed in the batch anyway: it keeps its own place when it comes first
			}
		}
		out = out.without(sl).with(aura)
		gone = gone.with(aura)
	}
	return out
}

// umbraOrder records the Auras an umbra armor answer destroyed in their
// hosts' place, so the departure order build can put each at its host's
// position in that seat's battlefield walk (gorge's replaceSBADestruction /
// effDestroyAll destroys the Aura while it processes that victim). Fixed
// size: no State field and no allocation.
//
// victim holds the recorded Auras that were also victims of the batch: gorge's
// victim loop reaches such an Aura at its own place when that comes before its
// host's (replaceUmbraArmor then leaves the already-moved Aura alone), so only
// a host that comes first takes it (umbraMoves).
type umbraOrder struct {
	pairs  [MaxPerm][2]Slot
	n      int
	victim slotMask
}

// add records that host was saved by destroying aura.
func (u *umbraOrder) add(host, aura Slot) {
	u.pairs[u.n] = [2]Slot{host, aura}
	u.n++
}

// auraOf returns the Aura an umbra armor answer destroyed in host's place,
// when the order build puts it there (umbraMoves).
func (s *State) auraOf(u *umbraOrder, host Slot) (Slot, bool) {
	for i := 0; i < u.n; i++ {
		if u.pairs[i][0] == host {
			return u.pairs[i][1], s.umbraMoves(u, i)
		}
	}
	return NoSlot, false
}

// umbraMoves reports whether pair i's Aura is emitted at its host's position:
// always, unless the Aura is itself a victim of the batch and gorge's victim
// walk (seat order, then battlefield order) reaches it before the host.
func (s *State) umbraMoves(u *umbraOrder, i int) bool {
	host, aura := u.pairs[i][0], u.pairs[i][1]
	if !u.victim.has(aura) {
		return true
	}
	ph, pa := s.perms[host].ctrl, s.perms[aura].ctrl
	if ph != pa {
		return ph < pa
	}
	var two [2]Slot
	s.battlefieldOrder(ph, slotBit(host).with(aura), two[:], 0)
	return two[0] == host
}

// DebugUmbraOwnSlotOrder (TestUmbraArmorGraveyardOrderMutation) restores the
// old order build: an umbra-spared Aura enters the graveyard at its own
// battlefield position instead of its host's.
var DebugUmbraOwnSlotOrder bool

// sbaSpare offers a lethal-damage casualty at sl to the shield counter and
// then to umbra armor (gorge replaceSBADestruction's order), true when one
// took the destruction. A shield counter leaves the damage marked, so the
// pass runs once more; an umbra Aura joins die, to be destroyed with the
// pass's other casualties, and is recorded so the order build puts it at
// sl's place in the walk.
func (s *State) sbaSpare(k *prep, sl Slot, die *slotMask, umbra *umbraOrder) bool {
	if k.gen != nil && k.gen.anyRepl&rbShield != 0 && s.shieldDestroy(k, sl) {
		s.cfgx |= cfgxShieldPass // CR 122.1c: the counter goes instead, the damage stays marked for the next pass
		return true
	}
	if aura := s.umbraBatch(k, sl); aura != NoSlot {
		*die = die.with(aura) // CR 702.89a: the damage left with the keyword's use
		umbra.add(sl, aura)
		return true
	}
	return false
}

// shieldUmbraAllIn is OpDestroyAll's pass after regeneration: each victim
// holding a shield counter loses one instead, then each an umbra Aura answers
// keeps living and the Aura is destroyed instead; u records the umbra answers
// for the caller's departure walk.
func (s *State) shieldUmbraAllIn(k *prep, hit slotMask, u *umbraOrder) slotMask {
	if k.gen != nil && k.gen.anyRepl&rbShield != 0 {
		hit = s.shieldAll(k, hit) // CR 122.1c
	}
	return s.umbraAllIn(k, hit, u)
}

// shieldUmbraAllLeave runs OpDestroyAll's shield and umbra pass and moves the
// remaining victims, each umbra-spared Aura at its host's position (gorge's
// effDestroyAll destroys it while processing that victim).
func (s *State) shieldUmbraAllLeave(k *prep, hit slotMask) {
	var u umbraOrder
	hit = s.shieldUmbraAllIn(k, hit, &u)
	s.leaveAllUmbra(k, hit, kop.ZoneGraveyard, &u)
}
