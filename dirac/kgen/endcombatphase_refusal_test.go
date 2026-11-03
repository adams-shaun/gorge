package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestEndCombatPhaseIsRefusedByName: gorge ends the combat phase (CR 724.2),
// dirac's kernel has no op for it, so kgen refuses a card whose chain names
// the API instead of lowering the rest and skipping the phase change.
func TestEndCombatPhaseIsRefusedByName(t *testing.T) {
	halt := &cards.SA{Kind: "DB", API: "EndCombatPhase", Params: map[string]string{"Optional": "True"}}
	face := &cards.Face{
		Name: "Synthetic Ceasefire", ManaCost: "1 W", Types: []string{"Instant"},
		Abilities: []*cards.SA{{Kind: "SP", API: "UntapAll", Sub: halt,
			Params: map[string]string{"ValidCards": "Creature.YouCtrl", "SubAbility": "DBHalt"}}},
	}
	wantRefusal(t, lowerSweepSynthetic(t, face), "api EndCombatPhase")
}
