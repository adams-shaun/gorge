package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Exile-a-permanent costs from the battlefield (gorge cost.parse's
// exileBattlefieldCost part; CR 118.3, 602.2b).
//
// Exile<N/Spec> is the sacrifice's pick with the exile destination
// (kop.Cost.ExileBf on a CostSac part): the candidates are the payer's
// permanents matching Spec (a CantSacrifice static withholds none), each pick
// leaves to exile at payment and nothing is sacrificed. Exile<1/OriginalHost>
// is the granting host's exile (kop.CostHostExile), the one candidate settled
// unasked.

// pickExiles reports whether the cost being built pays its CostSac picks by
// exiling them (kop.Cost.ExileBf).
func (s *State) pickExiles(k *prep) bool {
	if s.escBfEntry(k) {
		// the printed escape cast's battlefield part: buildPickCost has moved on
		// to the grave exile by payment, the one CostSac pick is this part's
		return true
	}
	pc := s.buildPickCost(k)
	return pc != nil && pc.ExileBf
}

// escBfEntry reports whether the entry being built is a printed escape cast
// whose cost exiles a permanent beside the graveyard cards (CardDef.EscBfCost,
// Lunar Hatchling). gorge pays the exile parts in token order, the permanent
// first, so payPicks and paySacsGen pay the graveyard picks (payExilePicks)
// after the permanent's.
func (s *State) escBfEntry(k *prep) bool {
	e := &s.bld().e
	return e.kind == seSpell && e.trigPl == trigPlEscape && k.t.Cards[e.card].EscBfCost.ExileBf && !DebugEscapeSkipExiles
}

// countCostSac counts the permanents of p that can pay c's CostSac part: a
// sacrifice leaves out what a CantSacrifice static forbids, an exile does not.
func (s *State) countCostSac(k *prep, p uint8, c *kop.Cost, src Slot) int {
	return s.countPermMatch(k, p, c.Filter, src, !c.ExileBf)
}

// exileHostCost pays kop.CostHostExile: the permanent that granted the ability
// moves to exile as the cost is paid; its leaves-the-battlefield triggers fire
// and it is not sacrificed. Its last known information is kept as a
// sacrifice-this's is.
//
//go:noinline
func (s *State) exileHostCost(k *prep, e *stackEntry) {
	h := e.trigObj
	if h == NoObj || s.objs[h].zone != kop.ZoneBattlefield {
		return
	}
	sl := s.objs[h].slot
	if k.gen != nil {
		s.bld().e.li = s.snapLKI(h)
	}
	s.leave(k, sl, kop.ZoneExile, false)
}
