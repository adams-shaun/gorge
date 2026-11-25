package kgen

import "testing"

// playShapesCarriers is the round's slice of the gap-play-shapes carriers:
// the six gated-Play cards (the Condition*$ gate keys play's refusal loop
// once refused, now lowered by the node wrapper's SkipIf) and the four
// P1 Valid$-clause shapes playRememberedClause gained (Herald of Amity's
// Aura.IsRemembered, Lieutenant Nyota Uhura's comma alternatives, Flawless
// Forgery's ExiledWithSource beside IsRemembered), plus Eleven, the Mage,
// whose CheckSVar gate was the same class's only blocker. The umbrella's
// other classes (CastWithFlash shape, Play PlayCost, Play Valid$ without
// YouOwn/YouCtrl, Play Card.Self outside Amount$ All/ValidZone$ Exile, Play
// Amount "_", Play Defined$ "_") stay refused on their carriers (Boiling
// Rock Rioter and Sanwell, Avenger Ace since lowered: play_exilepop_lower_test.go;
// Plargg and Nassari: play_popcount_lower_test.go):
// Chandra Ablaze, Anrakyr the Traveller, Bismuth Mindrender,
// Blur of Heroism, Flash Photography, Harness the Storm, Jadzi, Oracle of
// Arcavios, Leonin Shikari, Mindleech Mass, Quantum
// Reduction, Silent-Blade Oni, Silver Scrutiny,
// Sorcerous Squall, Teferi Temporal Archmage, Teferi's Talent, The Face of
// Boe, The Infamous Cruelclaw, The Wandering Emperor, Timely Ward, Twinning
// Glass, Wizard's Spellbook, and each carries exactly its own class again.
var playShapesCarriers = []string{
	"Cemetery Tampering", "Coveted Prize", "Eleven, the Mage", "Flawless Forgery",
	"Herald of Amity", "Hidetsugu and Kairi", "Howltooth Hollow", "Lieutenant Nyota Uhura",
	"Rabble Rousing", "Wiretapping",
}

func TestPlayShapesSlice(t *testing.T) {
	for _, name := range playShapesCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
