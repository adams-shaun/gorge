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
// per ValidSA$ alternative. An AffectedZone$ Graveyard static with no
// scoping key lowers to "CantTgtGrave" (kop.StatCantTgtGrave, Ground Seal's
// class: every card in every graveyard untargetable, any source, any
// activator). Refused: any other AffectedZone$, EffectZone$ (a spell's own
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
		if az != "Graveyard" {
			return refuse("CantTarget AffectedZone %q", az)
		}
		// Ground Seal's class: the corpus's graveyard CantTarget carries no
		// scoping key at all (Cards in graveyards can't be the targets of
		// spells or abilities). gorge's matcher with no ValidTarget$ withholds
		// every card in the AffectedZone$ from any source and activator
		// (cantTargetBlocks), so the lowering is key-free too: anything besides
		// AffectedZone$ and Description$ is a different, unlowered shape.
		for _, k := range []string{"ValidTarget", "ValidSA", "ValidSource", "Activator", "Secondary"} {
			if _, ok := sp.get(k); ok {
				return refuse("CantTarget AffectedZone Graveyard beside %s$", k)
			}
		}
		sp.consume("AffectedZone")
		st.Kind = "CantTgtGrave" // build_ab.go: kop.StatCantTgtGrave (kw_canttgtgrave.go)
		return nil
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
