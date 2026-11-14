package deck

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestIsCommanderEligibleRejectsFlipBackHalf pins CR 903.3 against the
// eligibility loop reading every face: Nezumi Graverobber and Orochi
// Eggwatcher are flip cards whose FRONT faces ("Creature — Rat Rogue" /
// "Creature — Snake Shaman") are not legendary; only the flipped halves
// (Nighteyes the Desecrator, Shidako, Broodmistress) are legendary
// creatures. A flip card in the command zone is on its front face (CR 712.8a
// — the flipped half is what is in the command zone), so the front face is
// the one CR 903.3 judges. Reading the back half made both cards accepted as
// commanders.
func TestIsCommanderEligibleRejectsFlipBackHalf(t *testing.T) {
	cases := []struct {
		rel   string
		label string
	}{
		{"n/nezumi_graverobber_nighteyes_the_desecrator.txt", "Nezumi Graverobber"},
		{"o/orochi_eggwatcher_shidako_broodmistress.txt", "Orochi Eggwatcher"},
	}
	for _, tc := range cases {
		blob := corpusCardScript(t, tc.rel)
		c, diags := cards.ParseBytes(tc.label, blob)
		if len(diags) > 0 {
			t.Fatalf("%s parse: %v", tc.label, diags)
		}
		// Pin the precondition first: the front face is a NON-legendary
		// creature and the back face IS one. If corpus drift changes either
		// face, the eligibility assertion below stops meaning what it claims.
		if len(c.Faces) != 2 {
			t.Fatalf("%s has %d faces, want 2 (front + flip)", tc.label, len(c.Faces))
		}
		front, back := c.Faces[0], c.Faces[1]
		if front.IsLegendary() || !front.IsCreature() {
			t.Fatalf("fixture drift: %s front face Types %v; want a non-legendary creature", tc.label, front.Types)
		}
		if !back.IsLegendary() || !back.IsCreature() {
			t.Fatalf("fixture drift: %s back face Types %v; want a legendary creature", tc.label, back.Types)
		}
		if IsCommanderEligible(c) {
			t.Fatalf("IsCommanderEligible(%s) = true: its front face is not legendary and the flipped back face is not in the command zone (CR 903.3, CR 712.8a)", tc.label)
		}
	}
	// Positive control: a flip card that IS legendary on its front face stays
	// eligible — the fix must judge face 0, not reject multi-face cards.
	blob := corpusCardScript(t, "e/erayo_soratami_ascendant_erayos_essence.txt")
	c, diags := cards.ParseBytes("Erayo", blob)
	if len(diags) > 0 {
		t.Fatalf("Erayo parse: %v", diags)
	}
	if len(c.Faces) != 2 {
		t.Fatalf("Erayo has %d faces, want 2", len(c.Faces))
	}
	if !c.Faces[0].IsLegendary() || !c.Faces[0].IsCreature() {
		t.Fatalf("fixture drift: Erayo front face Types %v; want a legendary creature", c.Faces[0].Types)
	}
	if !IsCommanderEligible(c) {
		t.Fatal("IsCommanderEligible(Erayo, Soratami Ascendant) = false: a flip card legendary on its front face is a legal commander (CR 903.3)")
	}
}
