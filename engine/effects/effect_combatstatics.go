package effects

import (
	"github.com/adams-shaun/gorge/engine/state"
)

// Combat statics an api:Effect can deliver through StaticAbilities$, beside
// the printed S: route rules already reads for each.
const (
	// ModeBlockRestrict names CR 509.1b's ceiling on how many creatures a
	// defending player may block with ("each opponent can't block with more
	// than one creature this combat", Mirri, Weatherlight Duelist), read by
	// rules' combat.BlockRestrictLimit.
	ModeBlockRestrict = "BlockRestrict"
	// ModeAssignNoCombatDamage names CR 510.1's "assigns no combat damage"
	// (Ophidian's "If you do, Ophidian assigns no combat damage this turn",
	// the Laccolith family), read by rules' combat damage amount.
	ModeAssignNoCombatDamage = "AssignNoCombatDamage"
)

// effectCombatStatic registers an Effect-delivered combat static as a
// continuous restriction, reporting whether it did. A body carrying a key
// outside the read set fails closed to the caller's unimplemented Note,
// never to a blanket restriction. The lifetime is the Effect's own, the
// same read the AssignCombatDamageAsUnblocked arm makes: an absent
// Duration$ is this turn, UntilEndOfCombat ends with the combat phase.
func effectCombatStatic(h Host, c *Ctx, mode string, params staticLineParams, name, rawDur, dur string, remembered []state.ObjID) bool {
	switch {
	case mode == ModeBlockRestrict && staticKeysReadable(params, "Mode", "MaxBlockers", "ValidDefender", "Description"):
	case mode == ModeAssignNoCombatDamage && staticKeysReadable(params, "Mode", "ValidCard", "Description"):
	default:
		return false
	}
	effectContinuous(h, state.ContinuousEffect{
		Source: c.Source, Controller: c.Controller, Name: name,
		UntilEOT: effectUntilEOT(h, c.Source, rawDur), Duration: dur,
		Restriction: mode, RestrictParams: params,
		Remembered: remembered,
	})
	return true
}
