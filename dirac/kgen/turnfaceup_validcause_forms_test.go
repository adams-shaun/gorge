package kgen

import "testing"

// turnFaceUpValidCauseForms: Mode$ TurnFaceUp with ValidCause$
// SpellAbility.YouCtrl (gorge trigmatch turnFaceUpCauseAdmits) lowers as a
// gate on the turn-up's actor: the special action's turner or the resolving
// effect's controller. Growing Dread is the plain trigger; Oblivious Bookworm
// is the same line with Static$ True, which gorge resolves off the stack.
var turnFaceUpValidCauseForms = []formCase{
	{card: "Growing Dread", form: "TurnFaceUp ValidCause$ SpellAbility.YouCtrl", lir: "trigger TurnFaceUp"},
	{card: "Oblivious Bookworm", form: "TurnFaceUp ValidCause$ + Static$ True", lir: "trigger TurnFaceUp"},
}

func TestTurnFaceUpValidCauseForms(t *testing.T) { checkForms(t, turnFaceUpValidCauseForms) }
