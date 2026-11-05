package kgen

import (
	"strings"
	"testing"
)

// The Investigate shapes this ticket lowers: No Witnesses' Defined$
// Player.withMostTypeCreature (gorge playerHasMost TypeCreature) and a
// computed Num$ (Officious Interrogation's X = Count$Valid
// Creature.TargetedPlayerCtrl). Officious's spell still refuses at its
// relative RaiseCost ({W}{U} per target beyond the first), so only the
// Investigate node's refusal moved: the card's one refusal must be that
// RaiseCost, nothing about the Investigate node.
func TestInvestigateShapesLowering(t *testing.T) {
	lw := lowerNamedPool(t, []string{"No Witnesses", "Officious Interrogation"})
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatalf("buildPool: %v", err)
	}
	for _, c := range lw.cards {
		switch c.Name {
		case "No Witnesses":
			if len(c.Refusals) != 0 {
				t.Errorf("No Witnesses refused: %s", c.Refusals[0].Reason)
			}
		case "Officious Interrogation":
			if len(c.Refusals) != 1 {
				t.Fatalf("Officious Interrogation: %d refusals (%v)", len(c.Refusals), c.Refusals)
			}
			if !strings.HasPrefix(c.Refusals[0].Reason, "RaiseCost Cost$") {
				t.Errorf("Officious Interrogation: the Investigate node refused again: %s", c.Refusals[0].Reason)
			}
		}
	}
}
