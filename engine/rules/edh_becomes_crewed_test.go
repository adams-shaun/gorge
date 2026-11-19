package rules

// Mode$ BecomesCrewed (CR 702.122) -- "Whenever this Vehicle becomes crewed".
// The mode matches the events.CrewAction marker effects' Animate primitive
// emits once per crew ability RESOLUTION (CR 702.122e; the per-crewer Crew
// markers are the Creature.CrewedThisTurn filter's pairing and stay
// TriggerNone), so a Crew paid by several creatures queues the trigger once,
// not once per crewer. edh_becomes_crewed_resolution_test.go pins the timing.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// crewedVehicleFixture is the Mobilizer Mech round-trip's board: the corpus
// Avengers Quinjet (the "other target Vehicle you control" its trigger
// animates), the Mech itself and the two crew candidates for seat 0. It
// returns after the fixture's priority rounds, with the crew abilities
// offered.
func crewedVehicleFixture(t *testing.T, candidates ...string) (*Engine, state.ObjID, []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	quinjet, ok := reg.Lookup("Avengers Quinjet")
	if !ok {
		t.Fatal("corpus has no Avengers Quinjet")
	}
	extras := []*cards.Card{quinjet}
	for _, name := range candidates {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("corpus has no %s", name)
		}
		extras = append(extras, c)
	}
	e := corpusEngine(t, reg, extras, nil)
	quinjetID := moveByName(t, e, 0, "Avengers Quinjet", state.ZBattlefield)
	if o := e.G.Obj(quinjetID); o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("Avengers Quinjet precondition: zone %v tapped %v, want battlefield untapped", o.Zone, o.Tapped)
	}
	// The Quinjet's own ETB trigger fires on entering; answer it fully so the
	// assertions below see only the crew shape under test.
	e.pending = nil
	e.priorityRound()
	resolveQuinjetETB(t, e)
	e.pending = nil
	e.priorityRound()
	var ids []state.ObjID
	for _, name := range candidates {
		ids = append(ids, moveByName(t, e, 0, name, state.ZBattlefield))
	}
	e.pending = nil
	e.priorityRound()
	return e, quinjetID, ids
}

// TestEDHBecomesCrewedFiresOncePerActivation drives the printed text through
// a real game: Mobilizer Mech ("Whenever Mobilizer Mech becomes crewed, up to
// one other target Vehicle you control becomes an artifact creature until end
// of turn", Crew 3) is crewed by TWO creatures (Grizzly Bears 2 + Llanowar
// Elves 1), and the trigger must ask for its one target exactly ONCE -- not
// once per crewing creature -- and animate Avengers Quinjet until end of
// turn.
func TestEDHBecomesCrewedFiresOncePerActivation(t *testing.T) {
	t.Parallel()
	e, quinjet, ids := crewedVehicleFixture(t, "Mobilizer Mech", "Grizzly Bears", "Llanowar Elves")
	mech, bears, elves := ids[0], ids[1], ids[2]
	// Preconditions every assertion below leans on: the target Vehicle is a
	// plain non-creature artifact, the Mech is an unanimated Vehicle whose
	// crew floor the two candidates exactly reach with two creatures, and the
	// crew ability (the minted index-0 activation) is offered.
	if e.IsCreature(quinjet) {
		t.Fatal("precondition: Avengers Quinjet is already a creature; the animation would prove nothing")
	}
	if e.IsCreature(mech) {
		t.Fatal("precondition: Mobilizer Mech is already a creature")
	}
	if got := e.Power(mech); got != 3 || e.Power(bears) != 2 || e.Power(elves) != 1 {
		t.Fatalf("precondition powers: mech %d bears %d elves %d, want 3/2/1", e.Power(mech), e.Power(bears), e.Power(elves))
	}
	opt := abilityOption(t, e, mech, 0)
	if !strings.Contains(opt.Label, "Crew 3") {
		t.Fatalf("Mobilizer Mech ability label %q does not name Crew 3", opt.Label)
	}
	submitChoices(t, e, opt.Index)

	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.MinSum != 3 {
		t.Fatalf("crew tap election %+v, want KChoose MinSum 3", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{indexChoice(t, d, bears), indexChoice(t, d, elves)}}); err != nil {
		t.Fatalf("legal two-crewer election rejected: %v", err)
	}

	// Walk every decision to the empty stack (never past it: priority passes
	// at an empty stack advance the turn, which would run cleanup the
	// assertions do not care about), counting the trigger's own target
	// elections by their prompt: a once-per-crewer bug asks twice.
	targetAsks := 0
	for i := 0; i < 40; i++ {
		if len(e.G.Stack) == 0 {
			break
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the crew activation (stack depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KTarget && strings.Contains(d.Prompt, "Mobilizer Mech") {
			// The trigger's own target election ("up to one other target
			// Vehicle you control", its TgtPrompt prompt): count it -- a
			// once-per-crewer bug asks twice -- and aim it at the Quinjet.
			targetAsks++
			if targetAsks > 1 {
				t.Fatalf("Mobilizer Mech's BecomesCrewed trigger asked for a target %d times for one crew activation (one per crewer?)", targetAsks)
			}
			quinjetIdx := indexChoice(t, d, quinjet)
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{quinjetIdx}}); err != nil {
				t.Fatalf("legal trigger target election rejected: %v", err)
			}
			continue
		}
		if d.Kind == decision.KPriority {
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("pass priority: %v", err)
			}
			continue
		}
		t.Fatalf("unexpected decision %+v while resolving the BecomesCrewed trigger", d)
	}
	if targetAsks != 1 {
		t.Fatalf("Mobilizer Mech's BecomesCrewed trigger asked %d times, want exactly 1", targetAsks)
	}
	// The marker record: exactly one CrewAction for the Mech, carrying BOTH
	// crewers -- the per-crewer Crew markers stay one each.
	markers := crewActionMarkers(e, mech)
	if len(markers) != 1 || len(markers[0].IDs) != 2 {
		t.Fatalf("Mobilizer Mech's crew action markers %+v, want one with two crewers", markers)
	}
	if !e.IsCreature(quinjet) {
		t.Fatal("the trigger never animated the other target Vehicle")
	}
	// Until end of turn: on seat 0's next turn both animations have reverted.
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	if e.IsCreature(quinjet) {
		t.Fatal("the trigger's animation outlived end of turn")
	}
}

// indexChoice returns the option index naming obj in d, failing when absent.
func indexChoice(t *testing.T, d *decision.Decision, obj state.ObjID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == obj {
			return o.Index
		}
	}
	t.Fatalf("decision %+v offers no option for object %v", d, obj)
	return -1
}

// servantFixture is the Mighty Servant of Leuk-o round-trip's board: the
// Servant plus the named crew candidates, all on seat 0's battlefield.
func servantFixture(t *testing.T, candidates ...string) (*Engine, state.ObjID, []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	servant, ok := reg.Lookup("Mighty Servant of Leuk-o")
	if !ok {
		t.Fatal("corpus has no Mighty Servant of Leuk-o")
	}
	extras := []*cards.Card{servant}
	for _, name := range candidates {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("corpus has no %s", name)
		}
		extras = append(extras, c)
	}
	e := corpusEngine(t, reg, extras, nil)
	servantID := moveByName(t, e, 0, "Mighty Servant of Leuk-o", state.ZBattlefield)
	if o := e.G.Obj(servantID); o.Zone != state.ZBattlefield {
		t.Fatalf("Mighty Servant precondition: zone %v, want battlefield", o.Zone)
	}
	var ids []state.ObjID
	for _, name := range candidates {
		ids = append(ids, moveByName(t, e, 0, name, state.ZBattlefield))
	}
	e.pending = nil
	e.priorityRound()
	return e, servantID, ids
}

// crewActionMarkers collects the events.CrewAction markers the log carries
// for obj (the Vehicle that became crewed).
func crewActionMarkers(e *Engine, obj state.ObjID) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.CrewAction && ev.Obj == obj {
			out = append(out, ev)
		}
	}
	return out
}

// grantedTriggersOn reports how many granted triggers the continuous effects
// anchored to id carry: the Mighty Servant body's grant is a DB$ Animate
// Triggers$ rider, one AddTrigger continuous per named trigger.
func grantedTriggersOn(e *Engine, id state.ObjID) int {
	n := 0
	for i := range e.continuous {
		ce := e.continuous[i]
		if ce.Source == id && ce.AddTrigger != nil {
			n++
		}
	}
	return n
}

// crewServant activates the Servant's Crew 4 with the given crewers and pays
// the tap election through to the empty stack.
func crewServant(t *testing.T, e *Engine, servant state.ObjID, crewers ...state.ObjID) {
	t.Helper()
	submitChoices(t, e, abilityOption(t, e, servant, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("crew tap election %+v, want KChoose", d)
	}
	choice := make([]int, 0, len(crewers))
	for _, c := range crewers {
		choice = append(choice, indexChoice(t, d, c))
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choice}); err != nil {
		t.Fatalf("crew election %+v rejected: %v", choice, err)
	}
	passUntilStackEmpty(t, e, 30)
}

// TestEDHBecomesCrewedValidCrewAmountGatesTheTrigger pins both halves of
// Mighty Servant of Leuk-o's gate ("whenever it becomes crewed for the first
// time each turn, IF it was crewed by exactly two creatures, it gains ..."):
// a crewing by ONE creature (Mossbridge Troll's 5 power alone reaches Crew 4)
// grants nothing, and the SECOND crewing of the same turn grants nothing even
// though it was by exactly two creatures -- the first-time slot was spent by
// the first crewing (an intervening-if false when its event occurred does not
// save the trigger for a later event, CR 603.4).
func TestEDHBecomesCrewedValidCrewAmountGatesTheTrigger(t *testing.T) {
	t.Parallel()
	// Positive control: on a fresh board the FIRST crewing of the turn by
	// exactly two creatures DOES grant the trigger -- the absence assertions
	// below only mean something because this fires (and they fail with the
	// BecomesCrewed matcher reverted, which would grant nothing here).
	ctl, ctlServant, ctlIDs := servantFixture(t, "Grizzly Bears", "Grizzly Bears")
	crewServant(t, ctl, ctlServant, ctlIDs[0], ctlIDs[1])
	if n := grantedTriggersOn(ctl, ctlServant); n != 1 {
		t.Fatalf("control: a first two-crewer crewing granted %d triggers, want 1", n)
	}

	e, servant, ids := servantFixture(t, "Grizzly Bears", "Grizzly Bears", "Mossbridge Troll")
	bear1, bear2, troll := ids[0], ids[1], ids[2]
	// Preconditions: one crewer reaches Crew 4 alone (5 >= 4), two bears
	// reach it as exactly two creatures (2 + 2 = 4), and nothing is granted
	// yet.
	if e.Power(troll) != 5 || e.Power(bear1) != 2 || e.Power(bear2) != 2 {
		t.Fatalf("precondition powers: troll %d bears %d/%d, want 5/2/2", e.Power(troll), e.Power(bear1), e.Power(bear2))
	}
	if n := grantedTriggersOn(e, servant); n != 0 {
		t.Fatalf("precondition: the Servant carries %d granted triggers before any crewing", n)
	}

	// First crewing: ONE crewer. ValidCrewAmount$ 2 fails, so the trigger
	// must not fire -- and it spends the turn's first-time slot regardless.
	crewServant(t, e, servant, troll)
	if n := grantedTriggersOn(e, servant); n != 0 {
		t.Fatalf("a one-crewer crewing granted %d triggers, want none (ValidCrewAmount$ 2)", n)
	}
	markers := crewActionMarkers(e, servant)
	if len(markers) != 1 || len(markers[0].IDs) != 1 || markers[0].IDs[0] != troll {
		t.Fatalf("the one-crewer crewing's action markers %+v, want one carrying the Troll alone", markers)
	}

	// Second crewing, same turn: the Troll untapped in between and two
	// creatures reach Crew 4 (5 + 2 = 7). Exactly two crewers -- but not the
	// first crewing of the turn, so still nothing.
	e.pending = nil
	e.emit(events.Event{Kind: events.Untap, Obj: troll})
	e.priorityRound()
	if e.G.Obj(troll).Tapped {
		t.Fatal("precondition: the Troll did not untap between the two crewings")
	}
	crewServant(t, e, servant, troll, bear1)
	if n := grantedTriggersOn(e, servant); n != 0 {
		t.Fatalf("a second crewing of the turn granted %d triggers, want none (FirstTimeCrewed$ True)", n)
	}
	if markers := crewActionMarkers(e, servant); len(markers) != 2 || len(markers[1].IDs) != 2 {
		t.Fatalf("the two crewings' action markers %+v, want two, the second carrying two crewers", markers)
	}
}

// TestEDHBecomesCrewedFirstTimeGrantsOnTwoCrewers is the granting direction:
// the FIRST crewing of the turn, by exactly two creatures (the two bears, 2 +
// 2 = 4), fires the trigger once and its body grants the Servant the
// "whenever this creature deals combat damage to a player, draw two cards"
// trigger for the turn.
func TestEDHBecomesCrewedFirstTimeGrantsOnTwoCrewers(t *testing.T) {
	t.Parallel()
	e, servant, ids := servantFixture(t, "Grizzly Bears", "Grizzly Bears")
	bear1, bear2 := ids[0], ids[1]
	if e.Power(bear1)+e.Power(bear2) != 4 {
		t.Fatalf("precondition: bear powers %d + %d, want 2 + 2 = 4", e.Power(bear1), e.Power(bear2))
	}
	if n := grantedTriggersOn(e, servant); n != 0 {
		t.Fatalf("precondition: the Servant carries %d granted triggers before any crewing", n)
	}
	crewServant(t, e, servant, bear1, bear2)
	if n := grantedTriggersOn(e, servant); n != 1 {
		t.Fatalf("the first two-crewer crewing granted %d triggers, want exactly 1", n)
	}
	// The grant is the printed one: a combat-damage-to-a-player trigger whose
	// body draws two cards.
	var granted *cards.Trigger
	for i := range e.continuous {
		if ce := e.continuous[i]; ce.Source == servant && ce.AddTrigger != nil {
			granted = ce.AddTrigger
		}
	}
	if granted == nil || granted.Mode != "DamageDone" || granted.Params["CombatDamage"] != "True" ||
		granted.Params["ValidTarget"] != "Player" || granted.Params["Execute"] != "DBDraw" {
		t.Fatalf("granted trigger %+v is not \"whenever this creature deals combat damage to a player, draw ...\"", granted)
	}
	if body := e.G.Obj(servant).Face().SVars["DBDraw"]; !strings.Contains(body, "NumCards$ 2") {
		t.Fatalf("the granted trigger's body %q does not draw two cards", body)
	}
	if markers := crewActionMarkers(e, servant); len(markers) != 1 || len(markers[0].IDs) != 2 {
		t.Fatalf("the crewing's action markers %+v, want one carrying both bears", markers)
	}
}
