package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An attack duty beside an attack tax (CR 508.1d, 508.1g). A plain duty (an
// Effect's "attacks this turn if able": Incite, Heckling Fiends) is a Broad
// requirement, read like a MustAttack static (atkTaxFreePair). An Encore duty
// ("attacks that opponent this turn if able", kw_encore.go) names its
// defender: gorge's offer list (attackOffersCompute) scores a pair by the
// duties it satisfies and keeps a creature's pairs that satisfy at least as
// many as its best ZERO-charge pair, so
//
//   - with the named player's pair free (charge 0) only that pair survives (no
//     other player, planeswalker or battle) and the creature is required
//     (attackDutyDischargeable: that pair discharges the duty);
//   - with it charged (a Propaganda on the opponent it names) the best is 0:
//     every pair stays, the named one an optional pair, and the creature is not
//     required (a pair that costs mana never discharges a duty, CR 508.1d's last
//     sentence).
//
// A Broad requirement on the same creature (a MustAttack static) still
// requires it at any free pair (attackDutyDischargeable's Broad arm).

// DebugAtkTaxEncoreNamed (test only, TestFourSeatAtkTaxEncoreLockstep): an
// Encore creature is offered its named opponent alone and always required,
// as if that pair were free whatever the tax charges.
var DebugAtkTaxEncoreNamed bool

// atkTaxEncoreFree reports whether the Encore creature at sl has its named
// opponent's player pair free: offered (encoreSeat) and charged nothing.
func (s *State) atkTaxEncoreFree(k *prep, sl Slot) bool {
	q, ok := s.encoreSeat(k, sl)
	if !ok {
		return false
	}
	if DebugAtkTaxEncoreNamed {
		return true
	}
	n, priced := s.atkTaxCharge(k, sl, PlayerCode(q))
	return priced && n == 0 && !s.atkTaxDefenderBanned(k, sl, PlayerCode(q))
}

// atkTaxEncoreCharged reports whether the Encore creature at sl binds an
// opponent whose pair a tax charges: the duty then satisfies no free pair and
// binds no offer.
func (s *State) atkTaxEncoreCharged(k *prep, sl Slot) bool {
	if !atkTaxOn(k) {
		return false
	}
	if _, ok := s.encoreSeat(k, sl); !ok {
		return false
	}
	return !s.atkTaxEncoreFree(k, sl)
}

// atkTaxEncoreRequired is atkTaxFreePair for an Encore creature: required
// with its named pair free, else only through a Broad requirement of its own
// (a MustAttack static) at some free pair.
func (s *State) atkTaxEncoreRequired(k *prep, sl Slot) bool {
	if s.atkTaxEncoreFree(k, sl) {
		return true
	}
	return (s.statM.any() || k.embComb && s.anyEmb() || s.ovGateReach(k)) && s.staticFlag(k, kop.StatMustAttack, sl) && s.atkTaxFreePair(k, sl)
}
