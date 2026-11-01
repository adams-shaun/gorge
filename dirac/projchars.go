package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ProjChars is one battlefield object's derived characteristics for the
// oracle: the layer-4 types and subtypes,
// the layer-5 colours and the layer-6 keywords, compared beside the
// layer-7 P/T. Off the battlefield every field is zero.
type ProjChars struct {
	Types  kop.TypeMask // derived types (TToken excluded: ProjObj.Token carries it)
	Subs   kop.SubMask  // derived subtypes over Table.Subtypes
	AllSub bool         // every creature type, beyond the matchup's subtypes (an all-creature-types grant)
	Colors kop.ColorMask
	Kw     kop.KwMask
}

// ProjectChars fills p.Chars for every battlefield object of p (a
// projection Project wrote of s). It is the oracle's alone: no play or
// search path calls it, so Project and ProjectDelta stay as cheap as they
// were.
func (s *State) ProjectChars(p *Projection) {
	k := s.k()
	p.Chars = [MaxObj]ProjChars{}
	pm := s.occ
	if k.phasing {
		pm = pm.or(s.phasedSlots()) // gorge projects a phased-out permanent's characteristics too
	}
	for m := pm; m.any(); m = m.rest() {
		sl := m.first()
		o := s.perms[sl].obj
		if int(o) >= len(p.Chars) || s.objs[o].zone != kop.ZoneBattlefield && (!k.phasing || s.objs[o].zone < kop.ZonePhased) {
			continue
		}
		c := &p.Chars[o]
		c.Types = s.typesAt(k, sl) &^ kop.TToken
		c.Colors = s.colorsAt(k, sl) &^ kop.ColC
		c.Kw = s.kw(k, sl)
		if k.gen != nil {
			for i := range k.t.Subtypes {
				if i < kop.MaxSubtypes && s.hasSubGen(k, sl, uint8(i)) {
					c.Subs = c.Subs.With(i)
				}
			}
			c.AllSub = s.hasSubGen(k, sl, subOutside)
		} else {
			c.Subs = k.t.Cards[s.cardAt(sl)].Subtypes
		}
	}
}
