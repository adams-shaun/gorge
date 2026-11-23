package dirac

// ownTapBoundKept reports whether the gaps walk's affordability check for an
// activation whose cost taps the source (abilityAffords) leaves that source's
// own mana in the census. gorge's potential walk prices such an activation
// against its PotentialMana bound, which counts every untapped source,
// the activating land's own {B} included (Ifnir Deadlands' "{2}{B}{B}, {T},
// Sacrifice a Desert" beside Immersturm Skullcairn and an Incubation Druid,
// whose reflected Amount$ the bound reads as {C} alone: the black pips come
// from the Skullcairn and the land itself). The play is then insufficient and
// gorge's prefix search pays it from the Druid (the search skips the play's
// own {T} source: potential_script.go ownTapSource). The kernel's gaps walk
// keeps the land's unit on a board with a Combo Any / reflected Amount$ gap
// so the same play reaches caSearch, which taps the right sources, and on a
// board where a live granted mana ability sits beside the gap (caBesideGrant)
// so it reaches the grant search instead, which is gorge's one prefix search
// over both and prices the play exactly (mana_grantcomboany.go
// grantFirstGaps). The real walk (availX 0) still takes the source out: its
// planner cannot tap it twice.
func (s *State) ownTapBoundKept(k *prep, sl Slot) bool {
	return s.g.availX&availGaps != 0 && k.gen != nil && k.gen.anyComboAny &&
		(s.caBoard(k, s.perms[sl].ctrl) || s.caBesideGrant(k, s.perms[sl].ctrl))
}
