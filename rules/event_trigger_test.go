package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
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

func TestDiscardedTriggerValidCauseRejectsCosts(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	orvar := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Orvar, the All-Form"))
	var tr cards.Trigger
	for _, candidate := range e.G.Obj(orvar).Face().Triggers {
		if candidate.Mode == "Discarded" {
			tr = candidate
			break
		}
	}
	if tr.Effect == nil || tr.Params["ValidCause"] != "SpellAbility.OppCtrl" {
		t.Fatalf("unexpected Orvar discard trigger: %+v", tr)
	}
	ev := events.Event{Kind: events.MoveZone, Obj: orvar, From: state.ZHand, To: state.ZGraveyard, Text: "discarded as a cost"}
	if e.discardedMatches(tr, orvar, ev) {
		t.Fatal("Orvar matched a discard cost with no opposing spell or ability")
	}
	cause := e.G.AddObject(mustCorpusCard(t, reg, "Lightning Bolt"), 1)
	cause.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{cause.ID})
	if !e.discardedMatches(tr, orvar, ev) {
		t.Fatal("Orvar did not match a discard caused by an opponent spell")
	}
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

// An entry with Tapped$ True is a state of its zone change, not an event of
// becoming tapped. effects.ChangeZone records that distinction in the replayed
// Tap event so City of Brass cannot deal damage for entering tapped.
func TestTapsTriggerCityOfBrassDoesNotFireForEntryTapped(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	city := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "City of Brass"))
	e.emit(events.Event{Kind: events.Tap, Obj: city, Text: "entered tapped"})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("entering tapped queued %d Taps triggers, want 0", len(e.pendingTriggers))
	}
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

func TestTapsForManaTriggerForsakenMonumentRejectsWrongColourAndActor(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Forsaken Monument"))
	swamp := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Swamp"))
	e.resolveManaAbility(0, swamp, e.availableManaAbilities(0, swamp)[0], false)
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("black mana queued %d colourless-only triggers", len(e.pendingTriggers))
	}
	// Crypt Ghast's Activator$ You must not observe another player's land.
	e = layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Crypt Ghast"))
	swamp = onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Swamp"))
	e.resolveManaAbility(1, swamp, e.availableManaAbilities(1, swamp)[0], false)
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("opponent activation queued %d You-only triggers", len(e.pendingTriggers))
	}
}

func TestTapsForManaTriggerRegalBehemothChecksMonarch(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Regal Behemoth"))
	swamp := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Swamp"))
	e.resolveManaAbility(0, swamp, e.availableManaAbilities(0, swamp)[0], false)
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("non-monarch queued %d Regal Behemoth triggers", len(e.pendingTriggers))
	}
	e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
	e.emit(events.Event{Kind: events.Untap, Obj: swamp})
	e.resolveManaAbility(0, swamp, e.availableManaAbilities(0, swamp)[0], false)
	requireOneEventTrigger(t, e, "Regal Behemoth")
}

func TestAttackersDeclaredOneTargetTriggerHorizonExplorer(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Horizon Explorer"))
	attacker := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
	requireOneEventTrigger(t, e, "Horizon Explorer")
}

func TestAttackersDeclaredOneTargetKarazikarCarriesBothPlayers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	deck := mountainDeck(t, 40)
	e := New(Config{Seed: 1, Names: []string{"a", "b", "c"}, Decks: [][]*cards.Card{deck, deck, deck}})
	karazikar := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Karazikar, the Eye Tyrant"))
	attacker := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	hands := [2]int{len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1))}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 2, IDs: []state.ObjID{attacker}})
	requireOneEventTrigger(t, e, "Karazikar")
	e.putTriggersOnStack()
	e.resolveTop()
	for _, p := range []state.PlayerID{0, 1} {
		if got := e.G.Players[p].Life; got != 19 {
			t.Fatalf("seat %d life = %d, want 19", p, got)
		}
		if got, want := len(e.G.Zone(state.ZHand, p)), hands[p]; got != want+1 {
			t.Fatalf("seat %d hand = %d, want %d", p, got, want+1)
		}
	}

	// The other real trigger targets a creature controlled by the attacked
	// player; a seat-1 creature must never appear in this seat-2-only offer.
	e = New(Config{Seed: 1, Names: []string{"a", "b", "c"}, Decks: [][]*cards.Card{deck, deck, deck}})
	karazikar = onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Karazikar, the Eye Tyrant"))
	attacker = onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	wrong := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	right := onBoardCard(t, e, 2, mustCorpusCard(t, reg, "Grizzly Bears"))
	_ = karazikar
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 2, IDs: []state.ObjID{attacker}})
	e.putTriggersOnStack()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) != 1 || d.Options[0].Obj != right {
		t.Fatalf("Karazikar target options = %+v, want only attacked player's %d (not %d)", d, right, wrong)
	}
}
