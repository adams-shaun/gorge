package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwWithKeyword: FwWithKeyword, with<Keyword> over the printed keyword Arg
// (a KwID); gorge's objectHasKeyword (engine/effects/filter.go:986) reads the
// printed face and the keyword counters. A permanent reads nothing while face
// down (CR 708.2) or after a lose-all-abilities effect (the same gates as
// kop.W7WithInfect, filter_props.go). A layer-walk view fails closed, and so
// does a keyword the table keeps no printed record of (printedKeyword's
// default): kgen lowers only the keywords listed here (fwKeywordWords).
// Known limit: gorge's keywordInCtx also sees keywords a layer table derives
// or grants; the kernel reads the printed record (and the Horsemanship grant)
// only, so a lowered "gains toxic/cascade/convoke" effect would diverge. No
// such grant is lowered today.
//
//kmanifest:fword FwWithKeyword
func (s *State) fwWithKeyword(c *fctx, l *kop.FLit) bool {
	switch {
	case c.walk:
		return false
	case c.perm():
		cid := s.cardAt(c.sl)
		if cid == kop.NoCard || s.faceDown(c.k, c.sl) || s.perms[c.sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			return false
		}
		id := kop.KwID(l.Arg)
		if id == kop.KwHorsemanship && c.k.gen != nil && c.k.gen.anyEvasionGrant &&
			s.evasionGranted(c.k, c.sl)&kop.EvHorsemanship != 0 {
			return true // a granted horsemanship (kw_kwmark.go), as hasSkulkPerm reads skulk
		}
		return printedKeyword(&c.k.t.Cards[cid], id)
	case c.card():
		return printedKeyword(&c.k.t.Cards[c.c], kop.KwID(l.Arg))
	}
	return false
}

// printedKeyword reports whether the card prints keyword id, from the record
// the table keeps of it: false for a keyword with none.
func printedKeyword(cd *kop.CardDef, id kop.KwID) bool {
	switch id {
	case kop.KwToxic:
		return cd.Toxic > 0
	case kop.KwHorsemanship:
		return cd.Evasion&kop.EvHorsemanship != 0
	case kop.KwMorph:
		return cd.MorphFamily == kop.MorphMorph
	case kop.KwMegamorph:
		return cd.MorphFamily == kop.MorphMegamorph
	case kop.KwSuspend:
		return cd.Suspend > 0
	case kop.KwCascade:
		return cd.Cascade > 0
	case kop.KwConvoke:
		return cd.Convoke
	case kop.KwForetell:
		return cd.HasForetell
	case kop.KwMadness:
		return cd.HasMadness
	case kop.KwMultikicker:
		return cd.HasMultikicker
	}
	return false
}
