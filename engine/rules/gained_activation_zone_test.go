package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestGainedGraveyardActivationNotOfferedOnBattlefield: Marvin, Murderous
// Mimic has the activated abilities of the other creatures its controller
// controls. Resurrected Cultist's only one works from the graveyard
// (ActivationZone$ Graveyard), and a gained ability keeps that zone
// (CR 113.6, 602.2): Marvin is on the battlefield, so the activation must not
// be offered even with delirium and the mana to pay for it.
func TestGainedGraveyardActivationNotOfferedOnBattlefield(t *testing.T) {
	t.Parallel()
	e, _ := b5Engine(t, "Marvin, Murderous Mimic", "Resurrected Cultist", "Lightning Bolt", "Bonesplitter", "Grizzly Bears")
	searchMoveByName(t, e, "Lightning Bolt", state.ZGraveyard)
	searchMoveByName(t, e, "Bonesplitter", state.ZGraveyard)
	searchMoveByName(t, e, "Grizzly Bears", state.ZGraveyard)
	searchMoveByName(t, e, "Plains", state.ZGraveyard)
	searchMoveByName(t, e, "Resurrected Cultist", state.ZBattlefield)
	marvin := searchMoveByName(t, e, "Marvin, Murderous Mimic", state.ZBattlefield)
	addMana(t, e, 0, "BBBB")
	e.pending = nil
	e.priorityRound()
	if gainedAbilityOffered(e.Pending(), marvin) {
		t.Fatalf("Marvin was offered Resurrected Cultist's graveyard-only ability on the battlefield: %+v", e.Pending().Options)
	}
}
