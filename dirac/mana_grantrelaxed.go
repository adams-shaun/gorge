package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// grantRelaxedAffords is gorge's relaxed unpayability proof
// (potential_plan.go paymentPlanRelaxedAlternatives) for an activation
// grantCap's walk lists: the census gaps become free alternatives, but a
// granted mana ability keeps its spend restriction, so a restricted grant the
// activation may not spend (Mm'menon's "{T}: Add {U}. Spend this mana only to
// cast a spell from anywhere other than your hand" under an Equip) is no unit
// of it. A play the relaxed census cannot pay is proven insufficient and takes
// no prefix search, however a granted tap would untap a land (Nanoform
// Sentinel). m is the walk's avail mode. A cast, or an activation with its own
// {T} source, keeps the walk's answer.
func (s *State) grantRelaxedAffords(k *prep, p uint8, a Action, cost *kop.ManaCost, ex Slot, d payDesc, m uint8) bool {
	if a.Kind() != AkActivate || ex != NoSlot {
		return true
	}
	var av avail
	av.xd, av.xdOn = d, true
	s.g.availX = availPlain | m
	s.availFor(k, p, &av)
	s.g.availX = 0
	return av.affords(cost, pipMaskOf(cost), 0)
}
