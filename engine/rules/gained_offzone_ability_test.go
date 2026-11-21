package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestGainedOffZoneAbilityNotOffered: Necrotic Ooze has the activated
// abilities of creature cards in all graveyards. Blessed Ghoul's "{2}{W/B}:
// Return this card from your graveyard to your hand" names the graveyard as
// the only zone it functions from (ActivationZone$ Graveyard); the copy the
// Ooze holds lives on the battlefield, so it functions nowhere (CR 113.6b) and
// must not be offered even though its mana-only cost is payable.
func TestGainedOffZoneAbilityNotOffered(t *testing.T) {
	t.Parallel()
	e, _ := b5Engine(t, "Necrotic Ooze", "Blessed Ghoul", "Swamp", "Swamp", "Swamp", "Plains")
	ghoul := searchMoveByName(t, e, "Blessed Ghoul", state.ZGraveyard)
	ooze := searchMoveByName(t, e, "Necrotic Ooze", state.ZBattlefield)
	addMana(t, e, 0, "BBBW")
	if e.G.Obj(ghoul).Zone != state.ZGraveyard {
		t.Fatal("precondition: Blessed Ghoul must be in the graveyard")
	}
	if gainedAbilityOffered(e.Pending(), ooze) {
		t.Fatalf("Necrotic Ooze offered a gained graveyard-only ability: %+v", e.Pending().Options)
	}
}
