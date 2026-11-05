package kgen

import "testing"

// The Investigate shapes this ticket lowers: No Witnesses' Defined$
// Player.withMostTypeCreature (gorge playerHasMost TypeCreature) and a
// computed Num$ (Officious Interrogation's X = Count$Valid
// Creature.TargetedPlayerCtrl). Officious's relative RaiseCost ({W}{U} per target
// beyond the first) lowers too (TestRaiseCostPerTargetLowering), so the card
// has no refusal at all.
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
			if len(c.Refusals) != 0 {
				t.Fatalf("Officious Interrogation: %d refusals (%v)", len(c.Refusals), c.Refusals)
			}
		}
	}
}
