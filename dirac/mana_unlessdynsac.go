package dirac

// unlessDynSacGaps is the unless / ward window census's dynamic {T}, sacrifice
// arm, the kernel's mirror of gorge's unlessWindowUnits evaluated-amount
// self-sacrifice arm (Lotus Blossom's "{T}, Sacrifice Lotus Blossom: Add X
// mana of any one color, X its petal counters", Shrine of Boundless Growth's
// "{T}, Sacrifice it: Add {C} for each charge counter"; CR 605.3a's payment
// window). gorge prices such a source in its unless census when the paid
// census is on and its activation is a deterministic self-sacrifice whose
// evaluated amount is positive, so the pay mode is offered; the probe's gate
// here must admit the walk the same way, or the kernel poses decline only
// where gorge offers pay (Rupture Spire's "sacrifice it unless you pay {1}",
// decision 163 of the recorded pair). The spec the arm mirrors is
// Sac<1/CARDNAME>, which matches only the source itself (CR 201.5), so a twin
// of the source is no obstacle -- each twin is its own unit.

// unlessDynSacGaps reports whether p controls a dynamic self-sacrifice mana
// source the unless census prices: the table's dynamic-amount variants hold
// its ability, its evaluated production is positive, and its activation is
// live (an untapped original, or the copy's own).
func (s *State) unlessDynSacGaps(k *prep, p uint8) bool {
	if k.gen == nil || len(k.gen.varPC) == 0 {
		return false
	}
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCGen(k, sl)
		if !(pc.manaDyn && pc.manaSac) || pc.manaN <= 0 {
			continue
		}
		if pc.manaAb < 0 && s.perms[sl].ovr.flags&ovCopy == 0 {
			continue
		}
		if !s.canTapForManaPC(k, sl, pc) {
			continue
		}
		return true
	}
	return false
}
