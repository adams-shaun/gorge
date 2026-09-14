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

// observedTriggerCount counts a source's trigger exactly once whether it is
// still waiting in the trigger queue or has already been represented by the
// replayed TriggerPush event. Submit calls Advance, so effect/cleanup tests can
// legitimately observe either side of that placement boundary.
func observedTriggerCount(e *Engine, source state.ObjID) int {
	n := 0
	for _, pt := range e.pendingTriggers {
		if pt.Source == source {
			n++
		}
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == source {
			n++
		}
	}
	return n
}

// TestDiscardedTriggerNecropotenceFromResolvingEffect drives Mind Peel's real
// corpus Discard SA through resolveTop and its mid-resolution answer. This
// guards the producer boundary: effDiscard itself must mark the MoveZone, not
// rely on a test manufacturing a discard label.
func TestDiscardedTriggerNecropotenceFromResolvingEffect(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	necro := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Necropotence"))
	mindPeel := e.G.AddObject(mustCorpusCard(t, reg, "Mind Peel"), 1)
	mindPeel.Zone = state.ZStack
	mindPeel.Targets = []state.Target{{Player: 0, IsPlayer: true}}
	e.G.SetZone(state.ZStack, 0, []state.ObjID{mindPeel.ID})

	e.resolveTop()
	d := e.Pending()
	if d == nil || d.ResumeKind != "discard" || len(d.Options) == 0 {
		t.Fatalf("Mind Peel discard decision = %+v, want a real discard ask", d)
	}
	discarded := d.Options[0].Obj
	submitChoices(t, e, d.Options[0].Index)
	if got := observedTriggerCount(e, necro); got != 1 {
		t.Fatalf("Necropotence observed triggers = %d, want 1 from Mind Peel", got)
	}
	found := false
	for _, ev := range e.L.Events {
		if ev.Obj == discarded && events.IsDiscard(ev) && !events.IsDiscardCost(ev) {
			found = true
		}
	}
	if !found {
		t.Fatal("Mind Peel moved the chosen card without a canonical discard event")
	}
}

// TestDiscardedTriggerNecropotenceFromCleanup drives the CR 514.1 decision
// path. Cleanup is not an effect primitive, so it has its own discard producer
// and must carry the same action marker without masquerading as a cost.
func TestDiscardedTriggerNecropotenceFromCleanup(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	necro := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Necropotence"))
	e.G.Active = 0
	e.G.Step = state.StepCleanup
	extra := onHand(t, e, 0, "Name:Cleanup Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	e.priorityRound()
	submitDiscard(t, e, extra)
	if got := observedTriggerCount(e, necro); got != 1 {
		t.Fatalf("Necropotence observed triggers = %d, want 1 from cleanup discard", got)
	}
	found := false
	for _, ev := range e.L.Events {
		if ev.Obj == extra && events.IsDiscard(ev) && !events.IsDiscardCost(ev) {
			found = true
		}
	}
	if !found {
		t.Fatal("cleanup moved the chosen card without a canonical discard event")
	}
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
	cost := events.DiscardCost(orvar)
	if e.discardedMatches(tr, orvar, cost) {
		t.Fatal("Orvar matched a discard cost with no opposing spell or ability")
	}
	cause := e.G.AddObject(mustCorpusCard(t, reg, "Lightning Bolt"), 1)
	cause.Zone = state.ZStack
	e.G.SetZone(state.ZStack, 0, []state.ObjID{cause.ID})
	if e.discardedMatches(tr, orvar, cost) {
		t.Fatal("Orvar attributed a discard cost to an unrelated spell already on the stack")
	}
	if !e.discardedMatches(tr, orvar, events.Discard(orvar, 0)) {
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

func TestTapsForManaTriggerCryptGhastPaysForCastImmediately(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	spell := mustCorpusCard(t, reg, "Black Knight") // real {B}{B} corpus cost
	e := handEngine(t, spell)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Crypt Ghast"))
	swamp := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Swamp"))
	spellID := e.G.Zone(state.ZHand, 0)[0]

	// Exercise the real CR 601.2g payment window: one Swamp's printed ability
	// supplies only {B}, so this cast succeeds only if Crypt Ghast's triggered
	// mana ability resolves immediately under CR 605.3b and supplies the second.
	e.pending = nil
	e.beginCast(0, decision.Option{Kind: "cast", Obj: spellID})
	e.Advance()
	submitChoices(t, e, activateOption(t, e, swamp))
	if len(e.G.Stack) != 1 || e.G.Stack[0] != spellID || e.G.Obj(spellID).Zone != state.ZStack {
		t.Fatalf("Black Knight stack/zone = %v/%s, want paid cast on stack", e.G.Stack, e.G.Obj(spellID).Zone)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying {B}{B} = %+v (total %d), want empty", e.G.Players[0].Pool, got)
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("Crypt Ghast left %d pending triggers, want its mana trigger resolved without the stack", len(e.pendingTriggers))
	}

	// TapsForMana is an event mode, not itself a license to bypass the stack:
	// Manabarbs produces no mana and lacks Forge's triggered-mana marker.
	e = layerEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Manabarbs"))
	swamp = onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Swamp"))
	e.resolveManaAbility(0, swamp, e.availableManaAbilities(0, swamp)[0], false)
	requireOneEventTrigger(t, e, "Manabarbs")
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
	before := e.G.Players[0].Pool.Total()
	e.resolveManaAbility(0, swamp, e.availableManaAbilities(0, swamp)[0], false)
	if got := e.G.Players[0].Pool.Total() - before; got != 2 {
		t.Fatalf("monarch mana gained = %d, want Swamp mana plus immediate Regal Behemoth mana", got)
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("Regal Behemoth left %d pending triggers, want immediate mana resolution", len(e.pendingTriggers))
	}
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
