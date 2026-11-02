package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cold code, out of line.

// cardTypeBits are the CR 205.2a card types gorge's cardTypeWords counts on
// a battlefield face (instant and sorcery cannot be there). It includes
// Kindred (CR 308.3): gorge's cardTypeWords holds it, so the
// battlefield and graveyard folds share this mask.
const cardTypeBits = kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker | kop.TInstant | kop.TSorcery | kop.TBattle | kop.TKindred

// validAgg is ExValidAgg: gorge count_body_zone.go's zone-count fold over
// the battlefield objects matching F with a $<prop> suffix
// (zoneCountFold.visit): the extremes (GreatestCardPower/-Toughness/
// -ManaCost; 0 when nothing matches), the CardToughness/CardManaCost sums
// (derived toughness on the battlefield, extremePropertyValue), the
// DifferentCardPower distinct set (refPower, derived) and Colors (the
// matches' face colours, ColorMaskOf).
//
//go:noinline
func (s *State) validAgg(k *prep, x *kop.Expr, ctx evalCtx) int {
	best, n := 0, 0
	seen := false
	var col kop.ColorMask
	var pows [MaxPerm]int // distinct powers seen (at most one per slot)
	np := 0
	var names [MaxPerm]CardID // distinct names seen (AggDistinctNames)
	nn := 0
	var types kop.TypeMask // the matches' face card types (AggCardTypes)
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if !s.matchPermGen(k, x.F, sl, ctx.ctrl, ctx.srcSl, layerFull) {
			continue
		}
		v := 0
		switch x.N {
		case kop.AggGreatestPower, kop.AggDistinctPower:
			v, _ = s.ptGen(k, sl)
		case kop.AggGreatestToughness, kop.AggSumToughness:
			_, v = s.ptGen(k, sl)
		case kop.AggGreatestMV, kop.AggSumMV:
			if !s.faceDown(k, sl) && !s.cpxNoCost(k, sl) {
				v = int(k.cards[s.cardAt(sl)].total)
			} // else CR 708.2: a face-down permanent has no mana cost
		case kop.AggColors:
			if !s.faceDown(k, sl) { // CR 708.2: a face-down permanent is colourless
				col |= k.t.Cards[s.cardAt(sl)].Colors
			}
			continue
		case kop.AggDistinctNames:
			// CR 201.2: gorge zoneFold seenDiffNames[o.Face().Name]
			nc := s.nameCardAt(sl)
			dup := false
			for i := 0; i < nn; i++ {
				if names[i] == nc {
					dup = true
					break
				}
			}
			if !dup {
				names[nn] = nc
				nn++
			}
			continue
		case kop.AggCardTypes:
			// CR 205.2a: the face's printed card types (gorge o.Face().Types
			// in cardTypeWords), not the layer-derived ones; a face-down
			// permanent has none (CR 708.2)
			if !s.faceDown(k, sl) {
				types |= k.t.Cards[s.cardAt(sl)].Types
			}
			continue
		case kop.AggSumCounters:
			n += s.perms[sl].ctrN(s, int(x.Ctr)) // CR 122.1
			continue
		}
		switch x.N {
		case kop.AggSumToughness, kop.AggSumMV:
			n += v
		case kop.AggDistinctPower:
			dup := false
			for i := 0; i < np; i++ {
				if pows[i] == v {
					dup = true
					break
				}
			}
			if !dup {
				pows[np] = v
				np++
			}
		default:
			if !seen || v > best {
				best, seen = v, true
			}
		}
	}
	switch x.N {
	case kop.AggSumToughness, kop.AggSumMV, kop.AggSumCounters:
		return n
	case kop.AggDistinctPower:
		return np
	case kop.AggDistinctNames:
		return nn
	case kop.AggCardTypes:
		return bits.OnesCount16(uint16(types & cardTypeBits))
	case kop.AggColors:
		return bits.OnesCount8(uint8(col))
	}
	return best
}
