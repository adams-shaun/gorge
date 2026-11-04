package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCantTargetArtifactSourceAbilities pins Species 8472's "can't be the
// target of abilities from artifact sources your opponents control"
// (`ValidSource$ Artifact | ValidSA$ Activated,Triggered | Activator$
// Player.Opponent`): an opponent's artifact's activated ability and its
// triggered ability can't target it, while an opponent's non-artifact
// ability and its controller's own artifact ability can (CR 115.4, 604.1).
func TestCantTargetArtifactSourceAbilities(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	species := lookup(t, reg, "Species 8472")
	rod := lookup(t, reg, "Rod of Ruin")

	// An opponent's artifact's activated ability: withheld.
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{species}, []*cards.Card{rod})
	sp := moveByName(t, e, 0, "Species 8472", state.ZBattlefield)
	rodID := moveByName(t, e, 1, "Rod of Ruin", state.ZBattlefield)
	cantTargetOnBoard(t, e, sp)
	addMana(t, e, 1, "RRR")
	d := activateObjAbility(t, e, 1, rodID)
	if indexOfObjOption(d, sp) >= 0 {
		t.Fatalf("Species 8472 was offered to an opponent's artifact's activated ability: %+v", d.Options)
	}
	replayCheck(t, e, cfg)

	// An opponent's non-artifact activated ability: offered.
	e, cfg = corpusEngineCfg(t, reg, []*cards.Card{species}, []*cards.Card{lookup(t, reg, "Cunning Sparkmage")})
	sp = moveByName(t, e, 0, "Species 8472", state.ZBattlefield)
	spark := moveByName(t, e, 1, "Cunning Sparkmage", state.ZBattlefield)
	cantTargetOnBoard(t, e, sp)
	d = activateObjAbility(t, e, 1, spark)
	if indexOfObjOption(d, sp) < 0 {
		t.Fatalf("Species 8472 was withheld from an opponent's non-artifact ability: %+v", d.Options)
	}
	replayCheck(t, e, cfg)

	// Its controller's own artifact's ability: offered.
	e, cfg = corpusEngineCfg(t, reg, []*cards.Card{species, rod}, nil)
	sp = moveByName(t, e, 0, "Species 8472", state.ZBattlefield)
	rodID = moveByName(t, e, 0, "Rod of Ruin", state.ZBattlefield)
	cantTargetOnBoard(t, e, sp)
	addMana(t, e, 0, "RRR")
	d = activateObjAbility(t, e, 0, rodID)
	if indexOfObjOption(d, sp) < 0 {
		t.Fatalf("Species 8472's controller could not target it with its own artifact: %+v", d.Options)
	}
	replayCheck(t, e, cfg)

	// An opponent's artifact's triggered ability: withheld, the other
	// creature still offered.
	e, cfg = corpusEngineCfg(t, reg, []*cards.Card{species, lookup(t, reg, "Grizzly Bears")}, []*cards.Card{lookup(t, reg, "Meteor Golem")})
	sp = moveByName(t, e, 0, "Species 8472", state.ZBattlefield)
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	cantTargetOnBoard(t, e, sp)
	drainEtb(t, e) // Species 8472's own lifegain trigger
	moveByName(t, e, 1, "Meteor Golem", state.ZBattlefield)
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("precondition: Meteor Golem queued %d triggers, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Player != 1 {
		t.Fatalf("expected Meteor Golem's trigger target decision for seat 1, got %+v", d)
	}
	if indexOfObjOption(d, bears) < 0 {
		t.Fatalf("precondition: the trigger did not offer Grizzly Bears: %+v", d.Options)
	}
	if indexOfObjOption(d, sp) >= 0 {
		t.Fatalf("Species 8472 was offered to an opponent's artifact's triggered ability: %+v", d.Options)
	}
	replayCheck(t, e, cfg)
}
