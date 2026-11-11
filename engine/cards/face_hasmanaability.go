package cards

// HasManaAbility reports whether the printed face carries a mana ability: an
// activated one (CR 605.1a, the intrinsic basic-land ability included) or the
// triggered CR 605.1b form (a static TapsForMana/ManaAdded trigger whose
// effect chain produces mana). It is the one home of the characteristic,
// reached through effects.FaceHasManaAbility by the gorge filter predicate and
// by kgen's CardDef.HasManaAbility, so the two cannot drift. A nil face has
// none.
func (f *Face) HasManaAbility() bool {
	if f == nil {
		return false
	}
	if len(f.ManaAbilities()) != 0 || len(f.ManaReflectedAbilities()) != 0 {
		return true
	}
	for i := range f.Triggers {
		tr := &f.Triggers[i]
		if tr.Mode != "TapsForMana" && tr.Mode != "ManaAdded" {
			continue
		}
		if v := tr.ParamStr(PKStatic); v != "True" && v != "true" {
			continue
		}
		for sa, d := tr.Effect, 0; sa != nil && d < 32; sa, d = sa.Sub, d+1 {
			if sa.Kind == "DB" && (sa.API == "Mana" || sa.API == "ManaReflected") {
				return true
			}
		}
	}
	return false
}
