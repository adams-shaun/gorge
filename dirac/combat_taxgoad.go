package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A goad beside an attack tax (CR 701.15b, 508.1d). gorge's offer list
// (rules attackOffersCompute) admits each pair its own price fits, then keeps a
// goaded creature's pairs that satisfy at least as many player-attack duties
// as its best ZERO-charge pair: the goad counts for a player pair (never a
// planeswalker's or a battle's), and CR 508.1d's last sentence makes a charged
// pair never raise the bar. With a free player pair the creature's walker and
// battle pairs drop (players only, as without a tax); with none the best is 0
// and every admitted pair stays, walkers and battles included. The goad
// requires the creature to attack only while a free player pair exists
// (attackDutyDischargeable; a Broad requirement beside it still discharges at
// any free pair). A defender that goaded it, while another live seat did not,
// is no pair at all (GoadMayAttack, goadDefenderBanned).

// DebugAtkTaxGoadPlayersOnly (test only, TestFourSeatAtkTaxGoadLockstep): a
// goaded creature is always offered players only and always required, as if a
// free player pair existed whatever the tax charges.
var DebugAtkTaxGoadPlayersOnly bool

// atkTaxGoadFreePlayer reports whether the goaded creature at sl has an
// offered player pair that costs nothing: a live defender not banned by the
// goad, not forbidden by a CantAttack static or a sickness exemption, whose
// charge is zero and priceable.
func (s *State) atkTaxGoadFreePlayer(k *prep, sl Slot) bool {
	if DebugAtkTaxGoadPlayersOnly {
		return true
	}
	sick := s.atkHasteSick(k, sl)
	for q := uint8(0); q < s.nSeats(); q++ {
		if q == s.active || s.out(q) || s.goadDefenderBanned(k, sl, q) || s.cantAtkPairPlayer(k, sl, q) || sick && !s.atkHasteOK(k, sl, q) {
			continue
		}
		if n, ok := s.atkTaxCharge(k, sl, PlayerCode(q)); ok && n == 0 {
			return true
		}
	}
	return false
}

// goadPlayersOnly reports whether the creature at sl, goaded or not, is
// offered player pairs alone: goaded, and (under an attack tax) with a free
// player pair to set the bar.
func (s *State) goadPlayersOnly(k *prep, sl Slot, goaded bool) bool {
	return goaded && (!atkTaxOn(k) || s.atkTaxGoadFreePlayer(k, sl))
}

// atkTaxGoadRequired is atkTaxFreePair for a goaded creature: it must attack
// when a free player pair exists, or (a broad MustAttack beside the goad)
// when any free pair does.
func (s *State) atkTaxGoadRequired(k *prep, sl Slot) bool {
	if s.atkTaxGoadFreePlayer(k, sl) {
		return true
	}
	return (s.statM.any() || k.embComb && s.anyEmb() || s.ovGateReach(k)) && s.staticFlag(k, kop.StatMustAttack, sl) && s.atkTaxFreePair(k, sl)
}

// atkTaxDefenderBanned reports whether target tgt (a player code or a
// planeswalker or battle object) of the goaded creature at sl is a banned
// defender's (goadDefenderBanned names the defending seat).
func (s *State) atkTaxDefenderBanned(k *prep, sl Slot, tgt TCode) bool {
	if !goadOn(k) || !s.goadedAt(k, sl) {
		return false
	}
	if isPlayerOrNone(tgt) {
		return s.goadDefenderBanned(k, sl, CodeSeat(tgt))
	}
	return s.goadDefenderBanned(k, sl, s.perms[s.objs[tgt].slot].ctrl)
}
