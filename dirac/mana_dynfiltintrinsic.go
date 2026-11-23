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
// ability, or a fixed-amount filter, keeps its intrinsic unit; so does a filter
// whose amount is zero now (Cabal Coffers made a Forest, no Swamps out): gorge's
// census drops it (manaAbilityYieldsNothing, dynManaYieldsNothing), so its
// intrinsic unit stays an ordinary planner unit. FeatPlannedCast's stricter
// ungranted census, taken while this mask is live, also drops a fixed paid
// filter's unit (Flooded Grove) that gorge's planner still funds: the same rare
// trade-off as the grant and combo-dyn clauses beside it.
func (s *State) dynFiltIntrinsic(k *prep, p uint8) (m slotMask) {
	g := k.gen
	if g == nil || !g.blgOn || !g.anyDynFilt {
		return m
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		j := g.filt(c)
		if j < 0 {
			continue
		}
		if a := &k.t.Cards[c].Abilities[j]; a.AFlags&kop.AfManaDynamic != 0 && s.blgMask(k, sl) != 0 && !s.dynManaYieldsNothing(k, a, sl) {
			m = m.with(sl)
		}
	}
	return m
}
