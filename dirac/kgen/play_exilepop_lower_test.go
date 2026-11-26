package kgen

import (
	"strings"
	"testing"
)

// TestPlayExilePopShapes: the paid Play populations over exile lower
// (lower_playexilepop.go): Boiling Rock Rioter's ExiledWithSource Ally filter
// and Sanwell, Avenger Ace's dotted Artifact.Creature head lower whole;
// Nathan Drake, Treasure Hunter's paid remembered Play lowers and the card
// stays refused by its ManaConvert static only. Goliath Daydreamer's free
// exile filter with neither IsRemembered nor ExiledWithSource keeps its
// refusal.
func TestPlayExilePopShapes(t *testing.T) {
	for _, name := range []string{"Boiling Rock Rioter", "Sanwell, Avenger Ace"} {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
	c := lowerNamedCard(t, "Nathan Drake, Treasure Hunter")
	if len(c.Refusals) != 1 || !strings.Contains(c.Refusals[0].Reason, "ManaConvert") {
		t.Errorf("Nathan Drake, Treasure Hunter refusals = %v, want only the ManaConvert static", c.Refusals)
	}
	c = lowerNamedCard(t, "Goliath Daydreamer")
	if !hasRefusal(c.Refusals, "Play ValidZone$ Exile without IsRemembered") {
		t.Errorf("Goliath Daydreamer refusals = %v, want the exile-without-IsRemembered refusal", c.Refusals)
	}
}
