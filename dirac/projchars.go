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
			// copy exceptions are copiable values (CR 707.9b) and its
			// own rule-based type switches are derived outside the layer
			// walk (phasedTypes, kw_phasing.go), so both stay while it is
			// phased out
			fd := s.faceDown(k, sl)
			if fd {
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
				if f := &cd.CDAFace; f.Folded {
					// the CDA kgen folded into the record is one of its
					// statics, which gorge does not apply while it is
					// phased out (rules/layers.go)
					c.Types, c.Colors = f.Types&^kop.TToken, f.Colors&^kop.ColC
					c.Subs, c.AllSub = cd.PrintedSubtypes, f.AllCreatureTypes
				}
			}
			c.Types = s.phasedTypes(k, sl, c.Types)
			if s.fxM.has(sl) {
				c.Subs = s.copyExcSubs(k, sl, c.Subs)
				c.AllSub = s.copyExcAll(k, sl, c.AllSub)
				c.Colors = s.copyExcCol(k, sl, c.Colors) &^ kop.ColC
				c.Kw |= s.copyExcKw(k, sl)
			}
			if k.gen != nil && k.gen.anyBestow {
				// the bestow type switch is derived live state, not a
				// continuous effect, so it stands while phased out: the Aura
				// subtype comes with the Aura type, and the creature subtypes
				// go with the Creature type (CR 702.26b, 702.103b, 205.1a)
				c.Subs, c.AllSub = s.bestowSubs(k, sl, c.Subs, c.AllSub)
			}
			if k.gen != nil && k.gen.anyReconfigure {
				// the Reconfigure switch is derived live state too: while
				// attached, the creature subtypes go with the Creature type
				// (CR 702.26b, 702.151b, 205.1a)
				c.Subs, c.AllSub = s.reconfigureSubs(k, sl, c.Subs, c.AllSub)
			}
			continue
		}
		c.Types = s.typesAt(k, sl) &^ kop.TToken
		c.Colors = s.colorsAt(k, sl) &^ kop.ColC
		c.Kw = s.kw(k, sl)
		// Printed protection is CardDef.ProtColors, not a keyword bit; the
		// oracle drops gorge's printed "Protection from <colour>" spellings
		// (oracle/projchars.go), so a grant of that same colour must not add
		// the bit back here.
		c.Kw &^= protColourKw(k.t.Cards[s.cardAt(sl)].ProtColors)
		if k.gen != nil && k.gen.anyColGrant {
			c.Kw |= s.colProtKw(k, sl) // static_colourprot.go
		}
		if k.gen != nil && k.gen.anyProtGrant {
			c.Kw |= s.grantedProtColKw(k, sl) // kw_protection.go
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

// protColourKw is the keyword bits of protection from the colours in c: only
// black and red have KwMask bits (the kernel's printed protection is
// CardDef.ProtColors, read through the target, block and damage paths).
func protColourKw(c kop.ColorMask) kop.KwMask {
	var m kop.KwMask
	if c&kop.ColB != 0 {
		m |= kop.KwProtBlack
	}
	if c&kop.ColR != 0 {
		m |= kop.KwProtRed
	}
	return m
}
