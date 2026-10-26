package rules

import (
	"testing"
)

// TestWinnowingKeepsSharedTypeCreatures pins the type comparison behind
// Winnowing's SacrificeAll ValidCards$ Creature.IsImprinted: a creature
// sharing a creature type with its controller's chosen creature is not
// imprinted, so it survives, while the others are sacrificed.
func TestWinnowingKeepsSharedTypeCreatures(t *testing.T) {
	t.Parallel()
	runRememberScenario(t, `{"name":"winnowing-shared","setup":{"p0":{"battlefield":["Grizzly Bears","Runeclaw Bear","Hill Giant"],"hand":["Winnowing"]},"p1":{"battlefield":["Hill Giant","Llanowar Elves"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Winnowing","mana":"WWWWWW"},
	  {"op":"resolve","answers":[{"kind":"choose","pick":["p0:Grizzly Bears"]},{"kind":"choose","pick":["p1:Hill Giant"]}]}],
	 "expect":[{"card":"p0:Grizzly Bears","zone":"battlefield"},{"card":"p0:Runeclaw Bear","zone":"battlefield"},
	  {"card":"p0:Hill Giant","zone":"graveyard"},{"card":"p1:Hill Giant","zone":"battlefield"},
	  {"card":"p1:Llanowar Elves","zone":"graveyard"}]}`)
}
