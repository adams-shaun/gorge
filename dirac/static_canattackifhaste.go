package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// noHasteDef is atkHasteOK's defender argument for the defender-blind read.
const noHasteDef uint8 = 255

// The CanAttackIfHaste exemption (kop.StatCanAttackIfHaste; gorge
// combat.CanAttack and CanAttackPairAt over canAttackIfHaste): a
// summoning-sick creature without haste may be declared as an attacker as
// though it had haste. canAttack reads it defender-blind (any matching static,
// gorge canAttackIfHasteBlind), and the offers read it per defender seat, so a
// static scoped to the controller's opponents (StHasteOppDef) offers a sick
// creature against the opponents' players and permanents and withholds the
// controller's own pairs.

// atkHasteSick reports whether the creature at sl is held back by
// summoning sickness only through the exemption: a table with the static, the
// creature sick and without haste.
func (s *State) atkHasteSick(k *prep, sl Slot) bool {
	return k.gen != nil && k.gen.anyAtkHaste && s.perms[sl].flags&pfSick != 0 && s.kw(k, sl)&kop.KwHaste == 0
}

// atkHasteOK reports whether some CanAttackIfHaste static exempts the
// creature at sl against defender seat def (noHasteDef: any defender).
//
//go:noinline
func (s *State) atkHasteOK(k *prep, sl Slot, def uint8) bool {
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCanAttackIfHaste || !s.statCond(k, st) {
			return true
		}
		if f := a.Static.Filter; f != 0 && !s.matchPermGen(k, f, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		if a.Static.StFlags&kop.StHasteOppDef != 0 && def != noHasteDef && def == st.ctrl {
			return true
		}
		hit = true
		return false
	})
	return hit
}

// atkHasteNoPair reports whether the creature at sl, sick and without haste,
// is exempt against no live defender seat: gorge asks the attackers decision
// only when some creature has an offer (askAttackers' empty-offer path
// declares nothing), so such a creature alone does not open it.
func (s *State) atkHasteNoPair(k *prep, sl Slot) bool {
	if !s.atkHasteSick(k, sl) {
		return false
	}
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != s.active && !s.out(q) && s.atkHasteOK(k, sl, q) {
			return false
		}
	}
	return true
}
