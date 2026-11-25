package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A Taps watcher that defers a source beside a reflected mana ability (Plaza of
// Heroes under Blighted Burgeoning's "whenever enchanted land is tapped for
// mana"). gorge's interference walk (rules paymentPlanSourceInterference) asks
// the tap observers once per production of an ability (pay.PaymentPlanProductions);
// a reflected ability has none, so the walk never runs and that ability stays
// a TierNormal census unit while the source's {C} and restricted any-colour
// abilities are deferred. Its unit is the reflected colours alone (the {C} is
// the deferred ability's). Dirac defers the whole source, which left the
// census without it and refused plays gorge's plan pays with the reflected
// colour (Gate to the Afterlife with Plaza's {W} and a Mountain).

// deferredReflMask is the colours of the census unit gorge keeps for the
// Taps-deferred source at sl: its reflected colours, one mana. It is 0 for a
// source the deferral does not reach as a Taps watcher (a rider or split
// deferral has its own unit), for a lone reflected ability (reflTapsUnwatched
// keeps it an ordinary unit) and when no colour is reflected.
func (s *State) deferredReflMask(k *prep, p uint8, sl Slot) uint8 {
	c := s.cardAt(sl)
	if k.gen == nil || !k.gen.has(c, gfManaRefl) || s.reflTapsUnwatched(k, sl) || !s.tapsDeferred(k, p, sl) ||
		s.riderDeferred(k, sl) || s.splitDeferred(k, sl) || s.askColDeferred(k, sl) {
		return 0
	}
	return uint8(s.reflColours(k, sl, c)) &^ (1 << kop.C)
}

// availDeferredRefl adds that unit to a (availFor's deferred-source skip).
func (s *State) availDeferredRefl(k *prep, p uint8, sl Slot, a *avail) {
	if m := s.deferredReflMask(k, p, sl); m != 0 {
		a.addUnits(m, 1)
	}
}
