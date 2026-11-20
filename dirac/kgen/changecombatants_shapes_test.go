package kgen

import "testing"

// changeCombatantsForms covers api:ChangeCombatants. The Attacking$ True
// shape over one target lowers to the ChangeCombatants extension (Misleading
// Signpost, Portal Mage); every other corpus shape refuses by name so the
// census keeps a reason instead of a bare api blocker. Tuvok and Windshaper
// Planetar carry Optional$ True (gorge reads it as mandatory), Capricopian and
// Portal Manipulator carry an Attacking$ value gorge resolves as a Note, and
// Tahngarth's .Defending form is the same Note route.
var changeCombatantsForms = []formCase{
	{card: "Misleading Signpost", form: "Attacking$ True over one target", lir: "ChangeCombatants"},
	{card: "Portal Mage", form: "Attacking$ True over one target", lir: "ChangeCombatants"},
	{card: "Tuvok, Chief Tactical Officer", form: "Optional$ True", refused: "ChangeCombatants Optional$ True"},
	{card: "Windshaper Planetar", form: "Optional$ True", refused: "ChangeCombatants Optional$ True"},
	{card: "Capricopian", form: "Attacking$ Player.OpponentOf CardController", refused: "ChangeCombatants Attacking$ Player.OpponentOf CardController"},
	{card: "Portal Manipulator", form: "Attacking$ TargetedPlayer", refused: "ChangeCombatants Attacking$ TargetedPlayer"},
}

func TestChangeCombatantsShapes(t *testing.T) { checkForms(t, changeCombatantsForms) }

// TestIsGoadedDepartedObjectRefuses: Baeloth's dies trigger reads IsGoaded on
// the departed creature. gorge clears the goad on the zone move, so its match
// is false where the kernel would still read the goad (seed 30 of the four-seat
// combatantsgoad pod diverged); the card is refused by name until gorge's
// look-back is settled.
func TestIsGoadedDepartedObjectRefuses(t *testing.T) {
	checkForms(t, []formCase{{card: "Baeloth Barrityl, Entertainer", form: "IsGoaded in a Battlefield->Graveyard ValidCard$",
		refused: "IsGoaded on a departed object"}})
}
