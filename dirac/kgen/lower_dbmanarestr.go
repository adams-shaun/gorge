package kgen

// A resolving DB$ Mana with RestrictValid$ Spell.<spec>
// (Abstract Paintmage, Ashling Rekindled's back face, Desolation of Smaug;
// CR 106.6, 605.1a). gorge's effMana adds the units as ManaAdd events carrying
// the restriction, one restriction for every unit of a colour choice
// (askManaChoice picks first); the kernel floats them as restricted mana
// (State.rb) through AddMana F and ExtManaChoice F.

// dbManaRestrict reads a resolving DB$ Mana's RestrictValid$ (nil: none). Only
// Spell.<spec> alternatives lower; an Activated, Static or non-spell one keeps
// refusing by name.
func (lc *lowerCtx) dbManaRestrict(ps *params) (*lFilter, error) {
	rv, ok := ps.get("RestrictValid")
	if !ok {
		return nil, nil
	}
	var m lMana
	if err := lc.manaRestrict(rv, &m); err != nil {
		return nil, err
	}
	if m.Restrict == nil || m.RestrictAb != nil || m.NonSpell {
		return nil, refuse("DB$ Mana RestrictValid$ %q", rv)
	}
	return m.Restrict, nil
}
