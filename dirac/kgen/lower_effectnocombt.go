package kgen

import "strings"

// rememberedNoCombatDamage lowers an Effect-delivered AssignNoCombatDamage
// restriction over the object the Effect remembers (Cloak of Confusion,
// Laccolith Rig: RememberObjects$ TriggeredAttackerLKICopy, ValidCard$
// Card.IsRemembered, ForgetOnMoved$ Battlefield). The kernel's combat damage
// step zeroes the creature a StatNoCombatDamage static names (combatAmount),
// and an emblem has no source permanent, so the remembered object is the
// emblem's capture: the static's filter keeps only the creature check, the
// capture gate applies it to the remembered object alone, and
// ForgetOnMoved$ Battlefield lapses the capture when that object leaves
// (kop.Emblem.CaptureLapse). An absent Duration$ is this turn, the same EOT
// emblem the host-bound EffectSource shape lowers to.
func (lc *lowerCtx) rememberedNoCombatDamage(ps, sp *params, capture lRef) error {
	if strings.TrimSpace(ps.str("RememberObjects")) == "" {
		return refuse("Effect AssignNoCombatDamage without RememberObjects$")
	}
	if v := strings.TrimSpace(ps.str("ForgetOnMoved")); v != "Battlefield" {
		return refuse("Effect AssignNoCombatDamage ForgetOnMoved$ %q", v)
	}
	if v := strings.TrimSpace(sp.str("ValidCard")); v != "Card.IsRemembered" {
		return refuse("Effect AssignNoCombatDamage ValidCard$ %q", v)
	}
	if left := sp.unread(); len(left) > 0 {
		return refuse("Effect AssignNoCombatDamage unread %v", left)
	}
	switch capture.Kind {
	case rTriggered, rTarget, rRemembered:
	default:
		return refuse("Effect AssignNoCombatDamage over %s", capture)
	}
	eot, next, host, err := effectEmblemDuration(ps)
	if err != nil {
		return err
	}
	if host {
		return refuse("Effect AssignNoCombatDamage with a host-bound duration")
	}
	// the capturing emblem's Remembered test is the capture gate; the filter
	// keeps the creature check only (the pattern CanAttackDefender's
	// capturing emblem uses, lower_effectcaptured.go)
	f, err := lc.filter("Creature")
	if err != nil {
		return err
	}
	ps.consume("RememberObjects", "ForgetOnMoved", "Name", "Image", "Stackable")
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{
		Statics:  []lStatic{{Kind: "AssignNoCombatDamage", F: f}},
		UntilEOT: eot, UntilNextTurn: next,
		Capture: capture, CaptureSet: true, CaptureLapse: true,
	})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1})
	return nil
}
