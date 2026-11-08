package rules

import (
	"slices"
	"testing"
)

// TestFarmMalevolentChandelier pins DSK-s188 xe:RAND:1:18:w17 (and its sibling
// w27), the farm windows attributed to Malevolent Chandelier. The Chandelier
// only resolves in the window; the p1.graveyard difference came from the
// Mindskinner mill that follows its cast, which gorge once let reach the
// graveyard past Leyline of the Void's "exile it instead". Forge and XMage
// agree on the seven-card graveyard below, with the milled cards exiled.
func TestFarmMalevolentChandelier(t *testing.T) {
	t.Parallel()
	last := mindskinnerLastSnapshot(t, mindskinnerFarmWindowScenario)
	if !mindskinnerHasPermanent(last, "Malevolent Chandelier", 0) {
		t.Fatalf("precondition: Malevolent Chandelier did not resolve: %+v", last.Permanents)
	}
	if !mindskinnerHasPermanent(last, "The Mindskinner", 0) || !mindskinnerHasPermanent(last, "Leyline of the Void", 0) {
		t.Fatalf("precondition: replacement permanents missing from the final board: %+v", last.Permanents)
	}
	p1 := last.Players[1]
	if !mindskinnerContains(p1.Exile, "Seized from Slumber") {
		t.Fatalf("the milled Seized from Slumber was not exiled by Leyline of the Void: exile=%v graveyard=%v", p1.Exile, p1.Graveyard)
	}
	want := []string{"Break Down the Door", "Acrobatic Cheerleader", "Plains", "Exorcise", "Emerge from the Cocoon", "Split Up", "Enduring Innocence"}
	if !slices.Equal(p1.Graveyard, want) {
		t.Fatalf("p1.graveyard = %v, want Forge/XMage's %v", p1.Graveyard, want)
	}
}
