package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugRemLKIPrinted is the mutation: sacRemember accumulates
// the sacrificed card's PRINTED toughness instead of its derived
// last-known one. Tests only.
var DebugRemLKIPrinted bool

// remLKIPT is ExRemLKIPT (RememberedLKI$CardPower / $CardToughness over a
// RememberSacrificed$ set): the characteristic sum sacRemember accumulated
// before each permanent left. X.N is checked by kgen only (one program reads
// one of the two, and the Sacrifice sums that one).
func (s *State) remLKIPT() int { return int(s.res.remLK) }

// remLKAdd adds one sacrificed permanent's last-known power or toughness
// (the Sacrifice's B.N bits 2 and 4) to the resolution's sum, saturating at
// +-32767. fireSac runs it through sacRemember before the permanent moves.
//
//go:noinline
func (s *State) remLKAdd(k *prep, sl Slot, bits int) {
	pw, t := s.ptGen(k, sl)
	if bits&kop.SacSumMV != 0 {
		// Sacrificed$CardManaCost: the printed mana value,
		// the same read gorge's SacrificedInfo.ManaValue captured.
		pw = s.objMV(k, s.cardAt(sl), s.perms[sl].obj)
		if DebugCountHeads11Off {
			pw = 0
		}
	} else if bits&kop.SacSumToughness != 0 {
		pw = t
		if DebugRemLKIPrinted {
			pw = int(k.cards[s.cardAt(sl)].tough)
		}
	}
	v := int(s.res.remLK) + pw
	if v > 32767 {
		v = 32767
	} else if v < -32767 {
		v = -32767
	}
	s.res.remLK = int16(v)
}
