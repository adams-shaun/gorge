package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// dynFiltIntrinsic is p's sources whose own printed mana is a dynamic-amount
// paid filter (Cabal Coffers' "{2}, {T}: Add {B} for each Swamp you control")
// and that a granted basic land type gives an intrinsic "{T}: Add [c]" (CR
// 305.6; Realmwright making every land a Swamp). gorge's planner lists no unit
// for such a source: its census holds the filter as a gap (census_incomplete)
// and the source's intrinsic ability goes with it, so a cast the intrinsic mana
// alone completes is the prefix search's script, which the autopay surface
// never lists as a plain cast (FeatPlannedCast). A land with no printed
// ability, or a fixed-amount filter, keeps its intrinsic unit.
func (s *State) dynFiltIntrinsic(k *prep, p uint8) (m slotMask) {
	g := k.gen
	if g == nil || !g.blgOn || !g.anyDynFilt {
		return m
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if j := g.filt(c); j >= 0 && k.t.Cards[c].Abilities[j].AFlags&kop.AfManaDynamic != 0 && s.blgMask(k, sl) != 0 {
			m = m.with(sl)
		}
	}
	return m
}
