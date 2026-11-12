package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Suplex is the modal body of the DealDamage + `ReplaceDyingDefined$
// Targeted` shape: the damage mode's 3 damage is lethal to a 2/2, and the
// "instead of dying" rider must exile the targeted creature rather than let
// the state-based action put it in its owner's graveyard. The other targeted
// tests (Carbonize, the fight cards) pin the untargeted/self variants of the
// same shared replacement registration; this one pins the rider on a Charm
// mode, where the replacement is read from the mode body.
func TestSuplexDamageModeExilesTheCreatureInsteadOfDying(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	suplex := mustCorpusCard(t, reg, "Suplex")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	e, cfg := censusEngine(t, 8403, []*cards.Card{suplex}, []*cards.Card{bear})
	cardToHand(t, e, suplex)
	bearID := moveOwnerCard(t, e, 1, bear, state.ZBattlefield)
	// Precondition: the rule reads a creature on the battlefield, and the
	// mode's 3 damage is lethal to it, so "instead of dying" is the branch
	// actually exercised.
	if o := e.G.Obj(bearID); o.Zone != state.ZBattlefield || e.Toughness(bearID) != 2 {
		t.Fatalf("precondition: target bear zone %s toughness %d, want battlefield 2", o.Zone, e.Toughness(bearID))
	}
	addMana(t, e, 0, "1R")
	// Precondition: Suplex is castable with the one red mana plus generic.
	castFirst(t, e, "cast")
	d := e.Pending()
	submitChoices(t, e, modeOptionContaining(t, d, "deals 3 damage"))
	targetObject(t, e, bearID)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(bearID); o.Zone != state.ZExile {
		t.Fatalf("Suplex-lethal bear zone = %s, want exile", o.Zone)
	}
	replayCheck(t, e, cfg)
}
