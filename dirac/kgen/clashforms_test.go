package kgen

import "testing"

// clashForms: DB$ Clash (CR 701.31) lowers to one Clash instruction (both
// tops revealed, each player's top/bottom ask in APNAP order, the verdict kept
// in the resolution's result slot) followed by the WinSubAbility$ block.
var clashForms = []formCase{
	{card: "Adder-Staff Boggart", form: "ETB Clash, WinSubAbility$ PutCounter", lir: "Clash"},
	{card: "Bog Hoodlums", form: "ETB Clash, WinSubAbility$ PutCounter", lir: "Clash"},
	{card: "Broken Ambitions", form: "SubAbility$ Clash after a Counter, WinSubAbility$ Mill", lir: "Clash"},
	{card: "Fire Juggler", form: "AttackerBlocked Clash, WinSubAbility$ DamageAll", lir: "Clash"},
}

func TestClashForms(t *testing.T) { checkForms(t, clashForms) }
