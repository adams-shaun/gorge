package kgen

import "strings"

// effectCantBlockBy lowers a targeted (or remembered) Effect's CantBlockBy
// static over obj. Moved out of effectTargetStatic (its case grew the
// function past its complexity row): the shape checks, the ValidBlocker$
// ExHaste forms and the plain no-ValidBlocker$ grant
// (lower_cantblockplain.go) share the same refusal names as before.
func (lc *lowerCtx) effectCantBlockBy(ps *params, sp *params, obj lRef) error {
	// Creature.IsRemembered names the same attackers as Card.IsRemembered
	// (an attacker is a creature); Speed, Young Avenger spells it so.
	if sp.str("ValidAttacker") == "Creature.EffectSource" {
		return lc.cantBlockNamedBlocker(ps, sp, obj)
	}
	if va := sp.str("ValidAttacker"); va != "Card.IsRemembered" && va != "Creature.IsRemembered" {
		return refuse("Effect CantBlockBy shape")
	}
	exile, hasExile := ps.get("ExileOnMoved")
	exHaste := false
	exC := 0
	var exF *lFilter
	if vb, ok := sp.get("ValidBlocker"); ok {
		// Agility Bobblehead: "can't be blocked this turn except by
		// creatures with haste" (kernel ExtExHaste, the blocker's CURRENT
		// keywords like gorge's withoutHaste); Cavern Stomper / Cheeky
		// House-Mouse: Creature.power<op><N> (lower_cantblockpower.go, the
		// blocker's CURRENT power). The only ValidBlocker$ the kernel
		// holds besides the chosen-colour form (colorGrantEffect); the
		// ExileOnMoved$ Battlefield shape, or (Speed, Young Avenger) a
		// targeted creature with no ExileOnMoved$ and no Duration$: the
		// kernel record lapses with the target's object version, and ends
		// at the turn's end as the exile form does. gorge keeps the
		// remembered ObjID across a zone change, so for the haste and power
		// forms this is an approximation (a target
		// flickered back the same turn is still restricted in gorge); the
		// filter form refuses it below.
		if !hasExile && !lc.cantBlockTargetedUntilEOT(ps, obj) {
			return refuse("Effect CantBlockBy ValidBlocker %q", vb)
		}
		c, f, err := lc.cantBlockValidBlocker(vb)
		if err != nil {
			return err
		}
		if f != nil && !hasExile {
			// the filter form's record lapses with the target's object
			// version; gorge without ForgetOnMoved$ keeps matching the
			// remembered ObjID across a zone change (its Target carries no
			// Incarnation stamp), so only the ForgetOnMoved$ Battlefield
			// spelling (Kick in the Door) agrees with the kernel
			if fm, ok := ps.get("ForgetOnMoved"); !ok || strings.TrimSpace(fm) != "Battlefield" {
				return refuse("Effect CantBlockBy ValidBlocker %q", vb)
			}
		}
		exHaste, exC, exF = true, c, f
	}
	_, hasVB := sp.get("ValidBlocker")
	switch {
	case hasExile && strings.TrimSpace(exile) == "Battlefield":
		// ends for each permanent as it leaves (CR 611.2c), as a record does
	case !hasExile && exHaste:
		// the targeted ValidBlocker$ form checked above; a ForgetOnMoved$
		// Battlefield beside it (Kick in the Door) is the id lapse the
		// record already has (the target's new object is not the record's)
		if f, ok := ps.get("ForgetOnMoved"); ok && strings.TrimSpace(f) == "Battlefield" {
			ps.consume("ForgetOnMoved")
		}
	case !hasExile && !hasVB && lc.cantBlockPlainRemembered(ps, obj):
		// (lower_cantblockplain.go): the Unblockable pump template's
		// no-ValidBlocker$ static over the chain's own targets; the
		// keyword grant lapses with the target's departure, as gorge's
		// ForgetOnMoved$ Battlefield sweep does
		ps.consume("ForgetOnMoved")
	case !hasExile && lc.cantBlockAttackingTarget(ps, obj):
		// (Gossip's Talent) ExileOnMoved$ is
		// unread here: the kernel's grant lapses with the target's object
		// version, as the exile sweep ends it in gorge. The target is
		// attacking and the kernel has no extra combat, so "this turn"
		// reaches only this combat's declare-blockers step.
	default:
		return refuse("Effect CantBlockBy shape")
	}
	if ro := ps.str("RememberObjects"); ro != "" {
		switch ro {
		case "TriggeredAttacker":
			// gorge's effectRememberedCodes has no "TriggeredAttacker"
			// entry, so the nested Effect resolves with an empty
			// Remembered set and its CantBlockBy grants nothing (kc10b
			// Gravelgill Scoundrel / Tidal Terror); granting to the
			// trigger source would diverge.
			return refuse("Effect CantBlockBy RememberObjects %q", ro)
		case "TriggeredAttackerLKICopy":
			// gorge DOES remember this one: effectRememberedCodes maps
			// TriggeredAttackerLKICopy to RememberedLKI
			// (engine/effects/staticline.go:911 at the pin), so the
			// restriction grants unblockable. The kernel has no
			// remembered-LKI grant; do not lower it as a live object.
			return refuse("Effect CantBlockBy RememberObjects LKICopy")
		}
	}
	if err := lc.cantBlockGrant(ps, obj, exHaste, exC, exF); err != nil {
		return err
	}
	// the static's own unread parameters, as every effectTargetStatic case
	// leaves them
	if left := sp.unread(); len(left) > 0 {
		return refuse("Effect static unread %v", left)
	}
	return nil
}
