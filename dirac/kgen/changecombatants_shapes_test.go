package kgen

import "testing"

// changeCombatantsForms covers api:ChangeCombatants. The Attacking$ True
// shape over one target lowers to the ChangeCombatants extension (Misleading
// Signpost, Portal Mage); every other corpus shape refuses by name so the
// census keeps a reason instead of a bare api blocker. Tuvok and Windshaper
// Planetar carry Optional$ True (a per-attacker confirm the kernel does not
// lower), Capricopian and
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

// TestGoadedDiesTriggerLowers: Baeloth's dies trigger reads IsGoaded on the
// departing creature, and both sides read it the CR 603.10a way -- gorge
// matches the move on the last-known-information object's goad records
// (trigmatch/zone.go's look-back pass, StaticallyGoadedWithLKI for the static
// route), the kernel reads the goad the permanent still holds at its
// removal-point Dies firing. Seed 30 of the four-seat combatantsgoad pod once
// diverged on it, when gorge's look-back read the post-move object.
func TestGoadedDiesTriggerLowers(t *testing.T) {
	checkForms(t, []formCase{{card: "Baeloth Barrityl, Entertainer", form: "IsGoaded in a Battlefield->Graveyard ValidCard$",
		lir: "trigger Dies F={types=0x2 Attacking cmp(41 0 1"}})
}
