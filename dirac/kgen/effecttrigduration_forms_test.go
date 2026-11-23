package kgen

import "testing"

// effectTriggersDurationForms: an until-end-of-turn Effect's LifeGained
// Triggers$ lowers as the end-of-turn emblem (lower_zone.go emblemTimedMode:
// the player-event emblem walk, dropped at cleanup); the modes the timed
// emblem does not listen to keep refusing.
var effectTriggersDurationForms = []formCase{
	{card: "False Cure", form: "SP$ Effect Triggers$ LifeGained (until end of turn)", lir: "emblem[0] trigger=1 statics=0 until=EOT"},
	{card: "Vizkopa Guildmage", form: "AB$ Effect Triggers$ LifeGained ValidPlayer$ You (this turn)", lir: "emblem[0] trigger=2 statics=0 until=EOT"},
	{card: "Battle Cry", form: "SP$ Effect Triggers$ Blocks (until end of turn)", lir: "emblem[0] trigger=1 statics=0 until=EOT"},
	{card: "Melee", form: "Effect Duration$ UntilEndOfCombat", refused: "Effect triggers without a permanent duration"},
}

func TestEffectTriggersDurationForms(t *testing.T) { checkForms(t, effectTriggersDurationForms) }
