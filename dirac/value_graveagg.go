package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// the BLB reads. Cold code, out of line.

// graveAgg is ExGraveAgg: gorge count_zone_fold.go's zone-count fold over
// the graveyard cards matching F (Count$ValidGraveyard <spec>$<prop>), the
// mana-value properties only: GreatestCardManaCost (0 when nothing
// matches), the CardManaCost sum and the DifferentCardManaCost distinct
// set, and the card-type sets (graveTypeAgg). A card's mana value is state.ManaValue's off the battlefield: the
// printed one, a split card's halves summed (splitMV).
//
//go:noinline
func (s *State) graveAgg(k *prep, x *kop.Expr, ctx evalCtx) int {
	if x.N == kop.AggCardTypes || x.N == kop.AggPermTypes {
		return s.graveTypeAgg(k, x, ctx) // value_gravecounts.go
	}
	if x.N == kop.AggGreatestPower {
		return s.graveGreatestPower(k, x, ctx) // value_countheads.go
	}
	best, n := 0, 0
	var seen uint64
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		pl := &s.pl[pp]
		for j := 0; j < int(pl.graveN); j++ {
			o := pl.grave[j]
			if s.objs[o].of&ofToken != 0 {
				continue // a token waiting in a graveyard is no card (CR 111.7, 704.5d)
			}
			c := s.objs[o].card
			if !s.matchCardGenAt(k, x.F, c, pp, ctx.ctrl, o, &ctx) {
				continue
			}
			v := int(k.cards[c].total)
			if k.gen.anySplit {
				v = s.splitMV(k, c, o, v) // face_castface.go
			}
			switch x.N {
			case kop.AggSumMV:
				n += v
			case kop.AggDistinctMV:
				seen |= 1 << (uint(v) & 63)
			default: // AggGreatestMV
				if v > best {
					best = v
				}
			}
		}
	}
	switch x.N {
	case kop.AggSumMV:
		return n
	case kop.AggDistinctMV:
		return bits.OnesCount64(seen)
	}
	return best
}
