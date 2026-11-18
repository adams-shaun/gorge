package oracle

import "testing"

// CR 603.10a / 608.2h with CR 111.2: Gnome-Made Engine's enters trigger ("create
// a 1/1 Gnome ... TokenOwner$ TriggeredCardController") is controlled by the
// player who stole the Engine. That player then sacrifices it (to Titan Hunter)
// before the trigger resolves, so the Engine is back in its owner's graveyard.
// The token still belongs to the trigger's controller, the Engine's controller
// as it last existed on the battlefield. gorge read the card's live controller
// (its owner, once it left) and gave the Gnome to the wrong player
// (decision 855 of 855; tail-67-c000 pair 188).
func TestMBCTitanHunterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "etb_token_owner_survives_sacrificed_source")
}
