package kgen

import (
	"strings"
	"testing"
)

// TestPlayExilePopShapes: the paid Play populations over exile lower
// (lower_playexilepop.go): Boiling Rock Rioter's ExiledWithSource Ally filter
// and Sanwell, Avenger Ace's dotted Artifact.Creature head lower whole;
// Nathan Drake, Treasure Hunter's paid remembered Play lowers and the card
// stays refused by its ManaConvert static only.
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
}

// TestPlayExileFilterPopShapes: an exile filter population without IsRemembered
// lowers for a free play (lower_playexilepop.go): Krang & Shredder's
// ExiledWithSource nonland cast and Kylox, Visionary Inventor's imprinted
// instants and sorceries lower whole; Goliath Daydreamer's dream-counter
// filter lowers and is refused later, at emit, for its counter kind (the
// acceptance pin); Etrata, Deadly Fugitive its SetState
// static and Invasion of Alara its DigUntil ImprintFound$ shape.
func TestPlayExileFilterPopShapes(t *testing.T) {
	for _, name := range []string{"Krang & Shredder", "Kylox, Visionary Inventor"} {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
	for name, want := range map[string]string{
		"Etrata, Deadly Fugitive": "SetState",
		"Invasion of Alara":       "DigUntil ImprintFound$",
	} {
		c := lowerNamedCard(t, name)
		if hasRefusal(c.Refusals, "Play ValidZone$ Exile without IsRemembered") || !hasRefusal(c.Refusals, want) {
			t.Errorf("%s refusals = %v, want %q only", name, c.Refusals, want)
		}
	}
}
