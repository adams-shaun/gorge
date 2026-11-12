package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kw_canttgtgrave.go: a printed S:Mode$ CantTarget static whose AffectedZone$
// is Graveyard and which carries no other scoping key (Ground Seal, Silent
// Gravestone, Dennick, Underworld Cerberus; kop.StatCantTgtGrave, lowered in
// lower_canttarget.go). While the source is on the battlefield, cards in
// graveyards can't be the targets of spells or abilities (CR 115.4): gorge's
// matcher with no ValidTarget$ withholds every card in the AffectedZone$,
// from any source and any activator (engine/rules/canttarget_static.go
// cantTargetBlocks, read from the target offer and the CR 608.2b recheck).
// The kernel arms the gate at the three card-target readers, all of which
// route through targetOK (dirac/legal.go): the TgtCard branch, the
// TgtPermOrGraveCard grave arm and the TgtGraveExileCard grave arm, so the
// offer, the feasibility probe and the recheck all see it.
//
//go:noinline
func (s *State) graveCantTargetGen(k *prep) bool {
	if k.gen == nil || !k.gen.anyCantTgtGrave {
		return false
	}
	hit := false
	s.eachStatic(k, func(stc statSrc) bool {
		a := stc.a
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantTgtGrave {
			hit = true
			return false
		}
		return true
	})
	return hit
}
