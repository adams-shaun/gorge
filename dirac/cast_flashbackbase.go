package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// flashbackBase is the mana cost a flashback cast of spell entry e starts from,
// before the cost statics: the card's own flashback cost (a granted flashback
// costs the mana cost instead), or a live grant's own cost (kw_fbgrant.go).
// ok is false when e is not a flashback cast, so the printed cost stands. The
// build's cost and the target menu's per-candidate price (dropTargetsOwing)
// both read it, so a target-dependent raise (Charix, the Raging Isle over a
// flashback Sever the Bloodline) is priced against the cost the cast pays.
func (s *State) flashbackBase(k *prep, e *stackEntry) (kop.ManaCost, bool) {
	if e.flags&sfFlashback == 0 {
		return kop.ManaCost{}, false
	}
	cd := &k.t.Cards[e.card]
	if (cd.Keywords&kop.KwFlashback != 0 || cd.BeamMeUp) && e.flags&sfFbGranted == 0 || cd.Harmonize {
		return cd.AltCost, true
	}
	if k.gen != nil && k.gen.anyFbGrant && !cd.BeamMeUp && !cd.Aftermath {
		return s.fbGrantCost(k, e.ctrl, e.obj, s.bld().fbGrants != 0), true
	}
	return kop.ManaCost{}, false
}
