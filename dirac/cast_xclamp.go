package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The per-part bounds of a cast's or activation's announced {X}: the cost
// parts that also announce a count cap the X menu (CR 107.3, 601.2b). Cold
// code: buildStep reaches it only behind the announcing-part bits (cast.go's
// bsX gate).

// xPartClamps caps the X ask by each announcing part: PayLife<X> by life,
// Sac<X/Spec> by the sacrifice candidates, ExileFromGrave<X/Spec> by the
// graveyard's and Discard<X/Spec> by the hand's.
//
//go:noinline
func (s *State) xPartClamps(k *prep, cost *kop.ManaCost, mx int) int {
	if k.gen != nil {
		if k.gen.anyLifeX {
			mx = s.lifeXClamp(k, cost, mx)
		}
		if k.gen.anySacX {
			mx = s.sacXClamp(k, cost, mx)
		}
		if k.gen.anyExileX {
			mx = s.exileXClamp(k, cost, mx)
		}
	}
	if pc := s.buildPickCost(k); pc != nil && pc.DiscardX {
		mx = s.discardXClamp(k, cost, mx)
	}
	return mx
}

// xPartClampsHi is xPartClamps for the reduced-cost ceiling: the parts whose
// candidates can only cap the plain mana bound are re-clamped there.
//
//go:noinline
func (s *State) xPartClampsHi(k *prep, cost *kop.ManaCost, hi int) int {
	if k.gen != nil && k.gen.anyExileX {
		hi = s.exileXClamp(k, cost, hi)
	}
	if pc := s.buildPickCost(k); pc != nil && pc.DiscardX {
		hi = s.discardXClamp(k, cost, hi)
	}
	return hi
}
