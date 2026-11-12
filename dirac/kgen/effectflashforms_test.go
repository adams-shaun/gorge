package kgen

import "testing"

// effectflashForms: the Effect-delivered spell-flash grants and Teferi's
// sorcery-timing prohibition (gorge flash_effect_grants.go,
// castSorceryTimingExempt), one carrier per shape plus the shapes that stay
// refused, checked like effectstaticformsForms.
var effectflashForms = []formCase{
	{card: "Borne Upon a Wind", form: "Effect CastWithFlash over every spell, then a draw (an EOT emblem)", lir: "0 CreateEmblem obj=You\n     1 Draw obj=You A=1\n  emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Winding Canyons", form: "Effect CastWithFlash over creature spells with AffectedZone$ (read by neither engine)", lir: "emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Quicken", form: "Effect CastWithFlash with ForgetOnCast$ Sorcery.YouCtrl (the emblem's Filter2 ends it at the next sorcery cast)", lir: "emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Teferi, Time Raveler", form: "CantBeCast OnlySorcerySpeed$ True (kop CantCastSorcery) beside the +1's UntilYourNextTurn grant", lir: "flags=[ActivatorOpp SorcerySpeed]"},
	{card: "Teferi, Time Raveler", form: "the +1's emblem lasts until the controller's next turn", lir: "emblem[0] trigger=0 statics=1 until=next"},
	{card: "Complete the Circuit", form: "the grant beside a delayed copy trigger stays refused: no lockstep game casts it", refused: "its {5}{U} convoke cast is not reached by any lockstep game"},
	{card: "Teferi, Temporal Archmage", form: "the loyalty-timing emblem is a different shape and stays refused", refused: "CastWithFlash shape"},
}

func TestEffectFlashForms(t *testing.T) { checkForms(t, effectflashForms) }
