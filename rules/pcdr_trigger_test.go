package rules

// PlayerCountDefinedRegistered$<Property> carriers, end to end on the REAL
// corpus faces. The four cards are Knight of the Ebon Legion and Y'shtola,
// Night's Blessed (end-step `HighestLifeLostThisTurn` gates), Lost Monarch of
// Ifnir (second-main `HasPropertywasDealtCombatDamageThisTurnBy Zombie GE1`,
// backed by the engine's per-turn combat-damage ledger) and Ludevic,
// Necro-Alchemist (`HasPropertyLostLifeThisTurn` on the `.Other` group, the
// ConditionCheckSVar$ gate whose fail direction was OPEN).

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// pcdrEngine builds a two-seat game, seat 0 active (the CR 103.1 toss pinned
// via seatZeroStart), with each named REAL corpus card parked on seat 0's
// battlefield and no pending decision.
func pcdrEngine(t *testing.T, names ...string) (*Engine, map[string]state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	deck := mountainDeck(t, 40)
	for _, name := range names {
		card, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("corpus fixture: %s missing", name)
		}
		deck = append(deck, card)
	}
	e := New(seatZeroStart(Config{Seed: 42, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, mountainDeck(t, 41)}}))
	e.Advance()
	ids := map[string]state.ObjID{}
	for _, name := range names {
		ids[name] = crAbortMove(t, e, 0, name, state.ZBattlefield)
	}
	e.pending = nil
	return e, ids
}

// pcdrCombat drives ONE combat-damage assignment to a player through the real
// runCombatAssignments path, so the engine's per-turn combat-damage ledger is
// populated exactly as it is in a live attack.
func pcdrCombat(e *Engine, from state.ObjID, to state.PlayerID, amount int32) {
	e.combatRound.assignments = []assignment{{toPlayer: to, amount: amount, from: from}}
	e.runCombatAssignments()
}

// pcdrStep emits the StepChange the engine's own step advance would emit and
// clears any queued triggers.
func pcdrStep(e *Engine, s state.Step) {
	e.emit(events.Event{Kind: events.StepChange, Step: s})
	e.pendingTriggers = nil
}

// pcdrQueue emits a StepChange, counts the triggers queued for src and clears
// the queue -- the emit-then-observe-then-drain shape assertPhaseFires uses.
func pcdrQueue(e *Engine, s state.Step, src state.ObjID) int {
	e.emit(events.Event{Kind: events.StepChange, Step: s})
	n := queuedPhaseTriggers(e, src)
	e.pendingTriggers = nil
	return n
}

// TestLostMonarchOfIfnirZombieCombatDamageGatesMill pins the card's real
// intervening-if: its second-main trigger fires ONLY when a player was dealt
// combat damage by a Zombie this turn, and never for a non-Zombie source, for
// non-combat damage, or at the first main phase.
func TestLostMonarchOfIfnirZombieCombatDamageGatesMill(t *testing.T) {
	e, ids := pcdrEngine(t, "Lost Monarch of Ifnir", "Grizzly Bears")
	monarch := ids["Lost Monarch of Ifnir"]
	bears := ids["Grizzly Bears"]

	// First main never fires, and neither does the second when there is no
	// qualifying combat damage.
	if n := pcdrQueue(e, state.StepMain1, monarch); n != 0 {
		t.Fatalf("first main queued %d triggers with no Zombie damage, want 0", n)
	}
	if n := pcdrQueue(e, state.StepMain2, monarch); n != 0 {
		t.Fatalf("second main queued %d triggers with no Zombie damage, want 0", n)
	}

	// Non-combat damage from the Zombie itself: the ledger only records the
	// COMBAT damage site, so this must not satisfy the condition.
	e.damaging = monarch
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 3})
	e.damaging = 0
	if n := pcdrQueue(e, state.StepMain2, monarch); n != 0 {
		t.Fatalf("second main queued %d triggers for NON-combat Zombie damage, want 0", n)
	}

	// Combat damage from a non-Zombie: the source spec fails, no fire.
	pcdrCombat(e, bears, 1, 3)
	if n := pcdrQueue(e, state.StepMain2, monarch); n != 0 {
		t.Fatalf("second main queued %d triggers for a non-Zombie, want 0", n)
	}

	// Combat damage from the Zombie (the Monarch itself): the condition holds
	// and the second main fires. The first main still does not.
	pcdrCombat(e, monarch, 1, 3)
	if n := pcdrQueue(e, state.StepMain1, monarch); n != 0 {
		t.Fatalf("first main queued %d triggers after Zombie combat damage, want 0", n)
	}
	if n := pcdrQueue(e, state.StepMain2, monarch); n != 1 {
		t.Fatalf("second main queued %d triggers after Zombie combat damage, want 1", n)
	}
}

// TestLostMonarchOfIfnirTriggerMatchesWithCondition is the un-deleted variant
// of the Phase-gate pin: the FULL compiled trigger (including CheckSVar$ X)
// now matches exactly when the ledger holds a qualifying hit, at the second
// main phase only. Before the count head existed the condition could not be
// evaluated and triggerMatches returned false at every step.
func TestLostMonarchOfIfnirTriggerMatchesWithCondition(t *testing.T) {
	e, ids := pcdrEngine(t, "Lost Monarch of Ifnir")
	monarch := ids["Lost Monarch of Ifnir"]
	tr := crTriggerFixture(t, e, monarch, "Phase", "Mill")
	if tr.Params["CheckSVar"] == "" {
		t.Fatal("corpus fixture changed: the Lost Monarch trigger no longer carries CheckSVar$")
	}

	for s := state.Step(0); s <= state.StepCleanup; s++ {
		pcdrStep(e, s)
		if got := e.triggerMatches(tr, monarch, events.Event{Kind: events.StepChange, Step: s}, nil); got {
			t.Fatalf("step %s matched the full trigger with no qualifying damage", s)
		}
	}

	pcdrCombat(e, monarch, 1, 3)

	// Second main matches; the first main and every other step do not.
	sawSecondMain := false
	for s := state.Step(0); s <= state.StepCleanup; s++ {
		pcdrStep(e, s)
		got := e.triggerMatches(tr, monarch, events.Event{Kind: events.StepChange, Step: s}, nil)
		want := s == state.StepMain2
		if got != want {
			t.Fatalf("step %s: triggerMatches=%v, want %v (Zombie combat damage already dealt)", s, got, want)
		}
		if want {
			sawSecondMain = true
		}
	}
	if !sawSecondMain {
		t.Fatal("the second main phase never matched")
	}
}

// TestKnightOfTheEbonLegionEndStepLifeLossGate pins the end-step
// HighestLifeLostThisTurn gate: it fires only once a player has lost 4 or
// more life this turn.
func TestKnightOfTheEbonLegionEndStepLifeLossGate(t *testing.T) {
	e, ids := pcdrEngine(t, "Knight of the Ebon Legion")
	knight := ids["Knight of the Ebon Legion"]

	// 3 life lost this turn: below GE4, no fire.
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -3})
	if n := pcdrQueue(e, state.StepEnd, knight); n != 0 {
		t.Fatalf("end step queued %d triggers at 3 life lost, want 0", n)
	}

	// A fourth point of life lost pushes the extreme to 4: the gate holds.
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	if n := pcdrQueue(e, state.StepEnd, knight); n != 1 {
		t.Fatalf("end step queued %d triggers at 4 life lost, want 1", n)
	}
}

// TestYshtolaNightsBlessedEndStepLifeLossGate pins the same
// HighestLifeLostThisTurn route on the second carrier, whose end-step trigger
// fires for every player's end step (no ValidPlayer$).
func TestYshtolaNightsBlessedEndStepLifeLossGate(t *testing.T) {
	e, ids := pcdrEngine(t, "Y'shtola, Night's Blessed")
	yshtola := ids["Y'shtola, Night's Blessed"]

	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -3})
	if n := pcdrQueue(e, state.StepEnd, yshtola); n != 0 {
		t.Fatalf("end step queued %d triggers at 3 life lost, want 0", n)
	}

	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	if n := pcdrQueue(e, state.StepEnd, yshtola); n != 1 {
		t.Fatalf("end step queued %d triggers at 4 life lost, want 1", n)
	}
}

// TestLudevicOtherLostLifeConditionEvaluates pins the fail-direction
// correction on the shared ConditionCheckSVar$ evaluator: Ludevic's
// `OtherLost` body (`PlayerCountDefinedRegistered.Other$HasPropertyLostLifeThisTurn`)
// now RESOLVES, and holds exactly when a player other than the controller
// lost life this turn. Before the count head existed the body was unresolved,
// and the condition call site fails OPEN on an unresolved body — so the draw
// offer was made even when nobody but the controller lost life.
func TestLudevicOtherLostLifeConditionEvaluates(t *testing.T) {
	e, ids := pcdrEngine(t, "Ludevic, Necro-Alchemist")
	ludevic := ids["Ludevic, Necro-Alchemist"]
	face := e.G.Obj(ludevic).Face()
	if face.SVars["OtherLost"] == "" {
		t.Fatal("corpus fixture changed: Ludevic's OtherLost SVar is gone")
	}
	ctx := &effects.Ctx{Source: ludevic, Controller: 0, SVars: face.SVars}

	// Only the CONTROLLER lost life: resolves, and does not hold.
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -4})
	holds, evaluated := effects.CheckSVarHolds(e, ctx, "OtherLost", "")
	if !evaluated {
		t.Fatal("OtherLost is UNRESOLVED -- the fail-open direction the fix closes")
	}
	if holds {
		t.Fatal("OtherLost held with only the controller losing life")
	}

	// An opponent lost life: resolves and holds.
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -2})
	holds, evaluated = effects.CheckSVarHolds(e, ctx, "OtherLost", "")
	if !evaluated || !holds {
		t.Fatalf("OtherLost after an opponent lost life = (holds=%v, evaluated=%v), want (true, true)", holds, evaluated)
	}
}
