package dirac

// "you may play/cast <cards> from the top of your
// library". gorge's may-play walk (engine/rules/legal_mayplay.go) offers
// one library card: lib[0] of the priority player's own library, matched
// against the permission's filter like any graveyard or exile card (a land
// through the land walk, a nonland through the spell walk). The kgen
// lowering strips TopLibrary from the filter (implied by the zone), so the
// card offered here is the only one the Library permission can match. A
// cast pushes the card from the library (pushBuild: detach, prov =
// Library); a reversed cast returns it to the top (unpushSpell).

// DebugLibTopAnyCard is the mutation knob (TestLibtopMutation): tryLibTop
// offers the SECOND card from the top instead of the top one.
var DebugLibTopAnyCard bool

// tryLibTop offers player p's top library card to try.
func (s *State) tryLibTop(p uint8, try func(Obj)) {
	pl := &s.pl[p]
	i := pl.libLo
	if DebugLibTopAnyCard {
		i++
	}
	if i < pl.libHi {
		try(pl.lib[i])
	}
}
