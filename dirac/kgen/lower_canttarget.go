package kgen

import "strings"

// cantTarget lowers a printed S:Mode$ CantTarget (gorge cantTargetBlocks,
// engine/rules/canttarget_static.go): a battlefield permanent matching
// ValidTarget$ (read with the static's controller as "you" and its source
// as Self) can't be the target of a spell or ability whose source matches
// ValidSource$ (the spell card, or the permanent that has the ability; read
// from the same controller), whose kind is in ValidSA$ (Spell, Activated,
// Triggered; absent: every kind) and whose controller Activator$ names
// (Opponent: the static controller's opponents; You: the controller;
// absent: anyone). Flags: "ActivatorOpp", "ActivatorYou", and "SA<kind>"
// per ValidSA$ alternative. Refused: an AffectedZone$ other than the
// battlefield (Ground Seal reaches graveyards), EffectZone$ (a spell's own
// targeting rule), SourceCanOnlyTarget$ (unmodelled in gorge too), a
// ValidSA$ alternative with qualifiers, and a condition (no corpus
// CantTarget carries one; gorge's gate for one is not lockstep-verified).
func (lc *lowerCtx) cantTarget(sp *params, st *lStatic) error {
	if st.Cond.Kind != cNone {
		return refuse("CantTarget with a condition")
	}
	for _, k := range []string{"EffectZone", "SourceCanOnlyTarget", "ValidCard", "ValidCards", "ValidCreature"} {
		if _, ok := sp.get(k); ok {
			return refuse("CantTarget %s$", k)
		}
	}
	if az, ok := sp.get("AffectedZone"); ok && az != "Battlefield" {
		return refuse("CantTarget AffectedZone %q", az)
	}
	vt, ok := sp.get("ValidTarget")
	if !ok {
		return refuse("CantTarget without ValidTarget$")
	}
	var err error
	if st.F, err = lc.filter(vt); err != nil {
		return err
	}
	if st.F.Players != 0 {
		return refuse("CantTarget ValidTarget %q names a player", vt)
	}
	if vs, ok := sp.get("ValidSource"); ok {
		if st.F2, err = lc.filter(vs); err != nil {
			return err
		}
		if st.F2.Players != 0 {
			return refuse("CantTarget ValidSource %q names a player", vs)
		}
	}
	switch a := sp.str("Activator"); a {
	case "", "Player":
	case "You", "Player.You":
		st.Flags = append(st.Flags, "ActivatorYou")
	case "Opponent", "Player.Opponent":
		st.Flags = append(st.Flags, "ActivatorOpp")
	default:
		return refuse("CantTarget Activator %q", a)
	}
	if v, ok := sp.get("ValidSA"); ok {
		for _, alt := range strings.Split(v, ",") {
			switch alt = strings.TrimSpace(alt); alt {
			case "Spell", "Activated", "Triggered":
				st.Flags = append(st.Flags, "SA"+alt)
			default:
				return refuse("CantTarget ValidSA %q", alt)
			}
		}
	}
	sp.consume("Secondary")
	return nil
}
