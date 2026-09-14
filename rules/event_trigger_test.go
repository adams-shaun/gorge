package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// requireOneEventTrigger proves the real corpus T: line was queued. These
// tests deliberately stop at placement: the trigger effects themselves have
// independent primitive coverage, while this file pins the event boundary
// each Mode$ spelling observes.
func requireOneEventTrigger(t *testing.T, e *Engine, name string) {
	t.Helper()
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("%s queued %d triggers, want 1", name, len(e.pendingTriggers))
	}
}

func TestSacrificedTriggerMayhemDevil(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Mayhem Devil"))
	victim := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	e.emit(events.Event{Kind: events.MoveZone, Obj: victim, From: state.ZBattlefield,
		To: state.ZGraveyard, Text: "sacrificed"})
	requireOneEventTrigger(t, e, "Mayhem Devil")
}

func TestDiscardedTriggerNecropotence(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Necropotence"))
	discarded := e.G.AddObject(mustCorpusCard(t, reg, "Grizzly Bears"), 0)
	discarded.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), discarded.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: discarded.ID, From: state.ZHand,
		To: state.ZGraveyard, Text: "discarded as a cost"})
	requireOneEventTrigger(t, e, "Necropotence")
}

func TestCommitCrimeTriggerForsakenMiner(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	miner := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Forsaken Miner"))
	e.G.SetZone(state.ZBattlefield, 0, nil)
	e.G.Obj(miner).Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{miner})
	spell := e.G.AddObject(mustCorpusCard(t, reg, "Lightning Bolt"), 0)
	own := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	// A multi-target spell commits one crime, even if the criminal target is
	// appended after an innocent first target.
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: spell.ID, IDs: []state.ObjID{own}})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: spell.ID, Player: 1, Amount: 3})
	requireOneEventTrigger(t, e, "Forsaken Miner")
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: spell.ID, Player: 1, Amount: 3})
	requireOneEventTrigger(t, e, "Forsaken Miner")
}

func TestTapsTriggerCityOfBrass(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	city := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "City of Brass"))
	e.emit(events.Event{Kind: events.Tap, Obj: city})
	requireOneEventTrigger(t, e, "City of Brass")
}

func TestTapsForManaTriggerCryptGhast(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Crypt Ghast"))
	swamp := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Swamp"))
	abilities := e.availableManaAbilities(0, swamp)
	if len(abilities) != 1 {
		t.Fatalf("Swamp mana abilities = %d, want 1", len(abilities))
	}
	e.resolveManaAbility(0, swamp, abilities[0], false)
	requireOneEventTrigger(t, e, "Crypt Ghast")
}

func TestAttackersDeclaredOneTargetTriggerHorizonExplorer(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Horizon Explorer"))
	attacker := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
	requireOneEventTrigger(t, e, "Horizon Explorer")
}
