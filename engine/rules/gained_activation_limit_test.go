package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gainedLimitCarrierSrc is an AUTHORED has-all-abilities-of carrier with NO
// GainsAbilitiesLimitPerTurn$ (Patchwork Crawler's shape): any cap on the
// gained ability can only come from the foreign ability's own parameter.
func gainedLimitCarrierSrc(t testing.TB) *cards.Card {
	t.Helper()
	return card(t, "Name:Cage Keeper\nManaCost:2\nTypes:Creature\nPT:2/2\n"+
		"S:Mode$ Continuous | Affected$ Card.Self | EffectZone$ Battlefield | GainsAbilitiesOf$ Card.YouOwn+counters_GE1_CAGE | GainsAbilitiesOfZones$ Exile\n"+
		"Oracle:x\n")
}

// gainedLimitForeignSrc is the lent card: "{2}: Draw a card. Activate only
// once each turn." (Stromkirk Condemned's ActivationLimit$ 1 shape).
func gainedLimitForeignSrc(t testing.TB, limit string) *cards.Card {
	t.Helper()
	return card(t, "Name:Limited Grimoire\nManaCost:2\nTypes:Artifact\n"+
		"A:AB$ Draw | Cost$ 2 | NumCards$ 1 | "+limit+" | SpellDescription$ {2}: Draw a card.\n"+
		"Oracle:x\n")
}

func gainedLimitBoard(t *testing.T, limit string) (*Engine, Config, state.ObjID) {
	t.Helper()
	keeper := gainedLimitCarrierSrc(t)
	foreign := gainedLimitForeignSrc(t, limit)
	e, cfg := tokenReplGame(t, 9114, keeper, foreign)
	keeperID := moveSeededCard(t, e, 0, keeper, state.ZBattlefield)
	foreignID := moveSeededCard(t, e, 0, foreign, state.ZBattlefield)
	e.emit(events.Event{Kind: events.MoveZone, Obj: foreignID, From: state.ZBattlefield,
		To: state.ZExile, IDs: []state.ObjID{keeperID}})
	e.emit(events.Event{Kind: events.CounterChange, Obj: foreignID, Counter: "CAGE", Amount: 1})
	e.pending = nil
	e.priorityRound()
	driveToStep(t, e, 3, 0, state.StepMain1)
	addMana(t, e, 0, "CC")
	return e, cfg, keeperID
}

// TestGainedActivationLimitOncePerTurn: a lent "activate only once each turn"
// ability (CR 602.5b) is offered once and withheld for the rest of the turn.
func TestGainedActivationLimitOncePerTurn(t *testing.T) {
	t.Parallel()
	e, cfg, keeperID := gainedLimitBoard(t, "ActivationLimit$ 1")
	gained := gainedAbilitiesOn(e, keeperID)
	if len(gained) != 1 {
		t.Fatalf("precondition: gained options = %d, want the lent draw offered once", len(gained))
	}
	submitChoices(t, e, gained[0].Index)
	passUntilStackEmpty(t, e, 20)
	if n := countKind(e.L.Events, events.GainedAbilityPush, keeperID); n != 1 {
		t.Fatalf("GainedAbilityPush count = %d, want 1", n)
	}
	addMana(t, e, 0, "CC")
	e.priorityRound()
	if hasGainedAbility(e, keeperID) {
		t.Fatal("the lent ActivationLimit$ 1 ability is offered again the turn it was activated")
	}
	gainsDriveToStep(t, e, 5, 0, state.StepMain1)
	addMana(t, e, 0, "CC")
	if !hasGainedAbility(e, keeperID) {
		t.Fatal("the lent ability did not come back the next turn")
	}
	replayCheck(t, e, cfg)
}

// TestGainedActivationLimitGameWide: GameActivationLimit$ 1 ("activate only
// once") on the lent ability holds across turns.
func TestGainedActivationLimitGameWide(t *testing.T) {
	t.Parallel()
	e, cfg, keeperID := gainedLimitBoard(t, "GameActivationLimit$ 1")
	gained := gainedAbilitiesOn(e, keeperID)
	if len(gained) != 1 {
		t.Fatalf("precondition: gained options = %d, want 1", len(gained))
	}
	submitChoices(t, e, gained[0].Index)
	passUntilStackEmpty(t, e, 20)
	gainsDriveToStep(t, e, 5, 0, state.StepMain1)
	addMana(t, e, 0, "CC")
	if hasGainedAbility(e, keeperID) {
		t.Fatal("the lent GameActivationLimit$ 1 ability is offered again on a later turn")
	}
	replayCheck(t, e, cfg)
}
