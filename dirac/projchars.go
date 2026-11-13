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
		if int(o) >= len(p.Chars) || s.objs[o].zone != kop.ZoneBattlefield && (!k.phasing || !s.objs[o].zone.Phased()) {
			continue
		}
		c := &p.Chars[o]
		if k.phasing && s.hiddenAt(sl) {
			// no continuous effect applies to a phased-out permanent
			// (CR 702.26b; gorge matchesWithCharsPT), but its layer-1
			// copy exceptions are copiable values (CR 707.9b), so they
			// stay even while it is phased out
			if s.faceDown(k, sl) {
				// a face-down permanent stays face down while phased out
				// (CR 110.5, 708.2): a nameless, colourless 2/2 creature.
				// A copy of it still keeps its layer-1 copiable exceptions
				// below (CR 707.9b).
				c.Types = kop.TCreature
				if s.faceDownWard(k, sl) {
					c.Kw = kop.KwWard
				}
			} else {
				cd := &k.t.Cards[s.cardAt(sl)]
				c.Types, c.Colors, c.Kw = cd.Types&^kop.TToken, cd.Colors&^kop.ColC, cd.Keywords
				c.Subs, c.AllSub = cd.Subtypes, cd.AllCreatureTypes
			}
			if s.fxM.has(sl) {
				c.Types |= s.copyExcTypes(k, sl)
				if s.copyExcNoLegendary(k, sl) {
					c.Types &^= kop.TLegendary
				}
				c.Subs = c.Subs.Or(s.copyExcSubs(k, sl))
				c.AllSub = c.AllSub || s.copyExcAll(k, sl)
				c.Colors = s.copyExcCol(k, sl, c.Colors) &^ kop.ColC
				c.Kw |= s.copyExcKw(k, sl)
			}
			continue
		}
		c.Types = s.typesAt(k, sl) &^ kop.TToken
		c.Colors = s.colorsAt(k, sl) &^ kop.ColC
		c.Kw = s.kw(k, sl)
		if k.gen != nil && k.gen.anyColGrant {
			c.Kw |= s.colProtKw(k, sl) // static_colourprot.go
		}
		if k.gen != nil {
			// one batch query for the slot (subsGen), not one hasSubGen per
			// subtype: the per-slot reduction is i-independent
			subs, subsAll := s.subsGen(k, sl)
			for i := range k.t.Subtypes {
				if i < kop.MaxSubtypes && subs.Has(i) {
					c.Subs = c.Subs.With(i)
				}
			}
			c.AllSub = subsAll
		} else {
			c.Subs = k.t.Cards[s.cardAt(sl)].Subtypes
		}
	}
}
