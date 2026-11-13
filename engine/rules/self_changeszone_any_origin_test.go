package rules

import (
	"testing"
)

// TestUlamogMilledShufflesGraveyard pins CR 603.6a / 603.10a: Ulamog, the
// Infinite Gyre's "put into a graveyard from anywhere" line has no
// TriggerZones$, so milling it from the library must still fire it.
func TestUlamogMilledShufflesGraveyard(t *testing.T) {
	t.Parallel()
	runForEachScenario(t, `{"name":"ulamog-mill","cr":["603.6a","603.10a"],"why":"Ulamog milled from the library (XMage GrindstoneTest#testGrindstoneUlamog shape)","setup":{"p0":{"hand":["Thought Scour"],"battlefield":["Island"]},"p1":{"library_top":["Ulamog, the Infinite Gyre","Silvercoat Lion"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Thought Scour","mana":"U","targets":["p1"]},{"op":"resolve","seat":0},{"op":"resolve","seat":0}],"expect":[{"graveyard_size":{"p1":0}}]}`)
}

// TestGaeasBlessingMilledShuffles pins the Origin$ Library self line.
func TestGaeasBlessingMilledShuffles(t *testing.T) {
	t.Parallel()
	runForEachScenario(t, `{"name":"gaeas-blessing-mill","cr":["603.6a","603.10a"],"why":"Gaea's Blessing milled from the library shuffles the graveyard back","setup":{"p0":{"hand":["Thought Scour"],"battlefield":["Island"]},"p1":{"library_top":["Gaea's Blessing","Silvercoat Lion"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Thought Scour","mana":"U","targets":["p1"]},{"op":"resolve","seat":0},{"op":"resolve","seat":0}],"expect":[{"graveyard_size":{"p1":0}}]}`)
}

// TestDiesTriggerIgnoresMill pins that Origin$ Battlefield still gates the
// widened admission: Abyssal Gatekeeper milled from the library does not
// trigger its dies line, so both milled cards stay in the graveyard.
func TestDiesTriggerIgnoresMill(t *testing.T) {
	t.Parallel()
	runForEachScenario(t, `{"name":"dies-ignores-mill","cr":["603.6a"],"why":"a dies trigger does not fire when the card is milled","setup":{"p0":{"hand":["Thought Scour"],"battlefield":["Island"]},"p1":{"library_top":["Abyssal Gatekeeper","Silvercoat Lion"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Thought Scour","mana":"U","targets":["p1"]},{"op":"resolve","seat":0}],"expect":[{"graveyard_size":{"p1":2}}]}`)
}
