package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 603.6a: "Each time an event puts one or more permanents onto the
// battlefield, all permanents on the battlefield (including the newcomers)
// are checked for any enters-the-battlefield triggers that match the event."
// Permanents a single effect puts onto the battlefield enter together, so an
// entering observer ("whenever another creature you control enters") sees
// every OTHER member of its own batch, whatever order the effect moved them
// in. All cards are real corpus cards; no Forge script text is committed.

// cr603Resolve casts name from seat 0's hand with symbols and drives the game
// until the stack is empty and priority is back, answering every trigger-order
// ask with the offered order (no ordering is under test).
func cr603Resolve(t *testing.T, e *Engine, name, symbols string) {
	t.Helper()
	reg := searchTestRegistry(t)
	d := castSpellNamed(t, e, reg, name, symbols)
	for i := 0; i < 40; i++ {
		if d == nil {
			t.Fatalf("CR 603.6a %s seq %d: no decision pending", name, len(e.L.Events))
		}
		switch {
		case d.Kind == decision.KTriggerOrder:
			answerTriggerOrders(t, e)
		case d.Kind == decision.KPriority && stackEmpty(e) && len(e.pendingTriggers) == 0:
			return
		case d.Kind != decision.KPriority:
			t.Fatalf("CR 603.6a %s seq %d: unexpected %s decision %+v", name, len(e.L.Events), d.Kind, d)
		}
		d = passUntilResolved(t, e, 40)
	}
	t.Fatalf("CR 603.6a %s seq %d: resolution did not settle", name, len(e.L.Events))
}

// TestCR603_6aRaiseThePastEntrantsSeeEachOther is the FDN repro: Raise the
// Past returns Savannah Lions and Skyknight Squire together (graveyard order
// Lions first). The Squire's "whenever another creature you control enters"
// must see the Lions, so it gets one +1/+1 counter. Guard (true before and
// after the fix): Soul Warden, already on the battlefield, sees both entrants.
func TestCR603_6aRaiseThePastEntrantsSeeEachOther(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Raise the Past", "Savannah Lions", "Skyknight Squire", "Soul Warden")
	searchMoveByName(t, e, "Soul Warden", state.ZBattlefield)
	lions := searchMoveByName(t, e, "Savannah Lions", state.ZGraveyard)
	squire := searchMoveByName(t, e, "Skyknight Squire", state.ZGraveyard)
	life0 := e.G.Players[0].Life

	cr603Resolve(t, e, "Raise the Past", "WWWW")

	for _, id := range []state.ObjID{lions, squire} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("CR 603.6a Raise the Past seq %d: object %d not returned to the battlefield", len(e.L.Events), id)
		}
	}
	if got := e.G.Players[0].Life; got != life0+2 {
		t.Fatalf("CR 603.6a Raise the Past seq %d: life %d, want %d (Soul Warden on the battlefield sees both entrants)", len(e.L.Events), got, life0+2)
	}
	if got := e.G.Obj(squire).Counter("P1P1"); got != 1 {
		t.Errorf("CR 603.6a Raise the Past seq %d: Skyknight Squire P1P1 = %d, want 1 (it entered together with Savannah Lions and must see it)", len(e.L.Events), got)
	}
	replayCheck(t, e, cfg)
}

// TestCR603_6aLivingDeathWardensSeeEachOther: Living Death's return is a
// multi-object ChangeZone (Defined$ Remembered), not a ChangeZoneAll. Two Soul
// Wardens returned together each see the other: +2 life, not +1.
func TestCR603_6aLivingDeathWardensSeeEachOther(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Living Death", "Soul Warden", "Soul Warden")
	w1 := searchMoveByName(t, e, "Soul Warden", state.ZGraveyard)
	w2 := searchMoveByName(t, e, "Soul Warden", state.ZGraveyard)
	if w1 == w2 {
		t.Fatalf("precondition: one Soul Warden moved twice")
	}
	life0 := e.G.Players[0].Life

	cr603Resolve(t, e, "Living Death", "BBBBB")

	for _, id := range []state.ObjID{w1, w2} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("CR 603.6a Living Death seq %d: Soul Warden %d not returned", len(e.L.Events), id)
		}
	}
	if got := e.G.Players[0].Life; got != life0+2 {
		t.Errorf("CR 603.6a Living Death seq %d: life %d, want %d (each Soul Warden sees the other enter)", len(e.L.Events), got, life0+2)
	}
	replayCheck(t, e, cfg)
}

// TestCR603_2cRaiseThePastIsOneEntryEvent: the returns are ONE event, so
// Bess, Soul Nourisher's "whenever one or more other creatures you control
// with base power and toughness 1/1 enter" (Mode$ ChangesZoneAll, no
// ActivationLimit$) triggers once for Soul Warden and Skyknight Squire
// together (CR 603.2c), not once per card.
func TestCR603_2cRaiseThePastIsOneEntryEvent(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Raise the Past", "Bess, Soul Nourisher", "Soul Warden", "Skyknight Squire")
	bess := searchMoveByName(t, e, "Bess, Soul Nourisher", state.ZBattlefield)
	searchMoveByName(t, e, "Soul Warden", state.ZGraveyard)
	searchMoveByName(t, e, "Skyknight Squire", state.ZGraveyard)

	cr603Resolve(t, e, "Raise the Past", "WWWW")

	if got := e.G.Obj(bess).Counter("P1P1"); got != 1 {
		t.Errorf("CR 603.2c Raise the Past seq %d: Bess P1P1 = %d, want 1 (one batch of 1/1s entered)", len(e.L.Events), got)
	}
	replayCheck(t, e, cfg)
}

// TestCR603_6aCopyTokenOwnEntryTriggerStillFires is the guard for a token
// copy entering inside CopyPermanent's batch (true before and after the
// fix): Abyssal Harvester exiles Lightshell Duo from the graveyard and makes
// a token copy of it, and the copy's own "when this enters, surveil 2" must
// still trigger. The copy is staged outside the battlefield before its entry
// MoveZone, and that staged object is no last-known information.
func TestCR603_6aCopyTokenOwnEntryTriggerStillFires(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Abyssal Harvester", "Lightshell Duo")
	harvester := searchMoveByName(t, e, "Abyssal Harvester", state.ZBattlefield)
	duo := searchMoveByName(t, e, "Lightshell Duo", state.ZBattlefield)
	// Put the Duo into the graveyard this turn (the ability's ValidTgts$
	// Creature.ThisTurnEntered), and let the Harvester tap this turn.
	e.emit(events.Event{Kind: events.MoveZone, Obj: duo, From: state.ZBattlefield, To: state.ZGraveyard})
	zallDrain(t, e)
	e.G.Obj(harvester).SummonSick = false
	e.emit(events.Event{Kind: events.Note, Obj: harvester, Text: "test: no summoning sickness"})
	e.pending = nil
	e.priorityRound()

	opt, ok := findAbilityOption(e, harvester, 0)
	if !ok {
		t.Fatalf("CR 603.6a Abyssal Harvester seq %d: ability not offered", len(e.L.Events))
	}
	submitChoices(t, e, opt.Index)
	surveil := false
	for i := 0; i < 20 && !surveil; i++ {
		d := e.Pending()
		switch {
		case d == nil:
			t.Fatalf("CR 603.6a Abyssal Harvester seq %d: no decision pending", len(e.L.Events))
		case d.Kind == decision.KTarget:
			idx := -1
			for _, o := range d.Options {
				if o.Obj == duo {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("CR 603.6a Abyssal Harvester seq %d: Lightshell Duo not offered: %+v", len(e.L.Events), d.Options)
			}
			submitChoices(t, e, idx)
		case d.Kind == decision.KArrange:
			surveil = true
		case d.Kind == decision.KPriority && stackEmpty(e) && len(e.pendingTriggers) == 0 && i > 0:
			i = 20
		case d.Kind == decision.KPriority:
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			submitChoices(t, e, pass)
		default:
			t.Fatalf("CR 603.6a Abyssal Harvester seq %d: unexpected %s decision", len(e.L.Events), d.Kind)
		}
	}
	if !surveil {
		t.Fatalf("CR 603.6a Abyssal Harvester seq %d: the token copy's own surveil 2 never triggered", len(e.L.Events))
	}
	// No replayCheck: the fixture clears summoning sickness in place, which
	// a log replay cannot rebuild.
}
