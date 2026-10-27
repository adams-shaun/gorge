package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// upkeepRegs returns the LIVE (e.active) registrations carrying Duration$
// UntilYourNextUpkeep.
func upkeepRegs(e *Engine) []ContinuousEffect {
	var out []ContinuousEffect
	for _, ce := range e.active() {
		if effects.IsUntilYourNextUpkeepDuration(ce.Duration) {
			out = append(out, ce)
		}
	}
	return out
}

// upkeepRawRegs counts the registrations in e.continuous regardless of
// liveness.
func upkeepRawRegs(e *Engine) int {
	n := 0
	for _, ce := range e.continuous {
		if effects.IsUntilYourNextUpkeepDuration(ce.Duration) {
			n++
		}
	}
	return n
}

// wantUpkeepRegs fails the test unless exactly n live UntilYourNextUpkeep
// registrations exist.
func wantUpkeepRegs(t *testing.T, e *Engine, n int, when string) {
	t.Helper()
	if got := len(upkeepRegs(e)); got != n {
		t.Fatalf("%s: %d live UntilYourNextUpkeep registrations, want %d", when, got, n)
	}
}

// activateElkinBottle activates Elkin Bottle's Dig ability and drains the
// stack, leaving exactly one UntilYourNextUpkeep Effect registered.
func activateElkinBottle(t *testing.T, e *Engine, bottle state.ObjID) {
	t.Helper()
	if z := e.G.Obj(bottle).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Elkin Bottle is in %v, want the battlefield", z)
	}
	addMana(t, e, 0, "CCC")
	submitChoices(t, e, abilityOption(t, e, bottle, 0).Index)
	passUntilStackEmpty(t, e, 50)
	wantUpkeepRegs(t, e, 1, "after activating Elkin Bottle")
	ce := upkeepRegs(e)[0]
	if ce.UntilEOT || ce.UntilTurn != 0 || ce.Permanent {
		t.Fatalf("precondition: the registration must carry no other lifetime: %+v", ce)
	}
}

func TestUntilYourNextUpkeepEffectStaticExpiresAtOwnUpkeep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Elkin Bottle")}, nil)
	bottle := moveByName(t, e, 0, "Elkin Bottle", state.ZBattlefield)
	activateElkinBottle(t, e, bottle)

	driveToStepAll(t, e, 2, 1, state.StepMain1)
	wantUpkeepRegs(t, e, 1, "turn 2 (survives this turn's cleanup)")

	driveToStepAll(t, e, 3, 0, state.StepUpkeep)
	if z := e.G.Obj(bottle).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Elkin Bottle is in %v, want the battlefield (expiry, not source-leaves)", z)
	}
	wantUpkeepRegs(t, e, 0, "turn 3 upkeep")
	replayCheck(t, e, cfg)
}

func TestUntilYourNextUpkeepEffectOutlivesItsSource(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Elkin Bottle")}, nil)
	bottle := moveByName(t, e, 0, "Elkin Bottle", state.ZBattlefield)
	activateElkinBottle(t, e, bottle)

	e.emit(events.Event{Kind: events.MoveZone, Obj: bottle, From: state.ZBattlefield, To: state.ZGraveyard})
	if z := e.G.Obj(bottle).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Elkin Bottle is in %v, want the graveyard", z)
	}
	if n := upkeepRawRegs(e); n != 1 {
		t.Fatalf("precondition: %d raw registrations after the source left, want 1", n)
	}
	wantUpkeepRegs(t, e, 1, "after the Bottle left")

	driveToStepAll(t, e, 2, 1, state.StepMain1)
	wantUpkeepRegs(t, e, 1, "turn 2")

	driveToStepAll(t, e, 3, 0, state.StepUpkeep)
	wantUpkeepRegs(t, e, 0, "turn 3 upkeep")
	if n := upkeepRawRegs(e); n != 0 {
		t.Fatalf("%d raw registrations left at turn 3 upkeep, want 0", n)
	}
	replayCheck(t, e, cfg)
}

func TestUntilYourNextUpkeepPumpExpiresAtOwnUpkeep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Gabriel Angelfire")}, nil)
	gid := moveByName(t, e, 0, "Gabriel Angelfire", state.ZBattlefield)
	if z := e.G.Obj(gid).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Gabriel is in %v, want the battlefield", z)
	}
	driveToStepAll(t, e, 3, 0, state.StepUpkeep)
	// Resolve the upkeep trigger: choose option 0 (Flying), pass until empty.
	for i := 0; i < 50; i++ {
		d := e.Pending()
		if d != nil && d.Kind == decision.KModes {
			submitChoices(t, e, 0)
			continue
		}
		if len(e.G.Stack) == 0 {
			break
		}
		submitPass(t, e)
	}
	if len(e.G.Stack) != 0 {
		t.Fatal("precondition: Gabriel's trigger did not resolve")
	}
	wantUpkeepRegs(t, e, 1, "after Gabriel's trigger")
	if upkeepRegs(e)[0].UntilEOT {
		t.Fatal("the pump must not be UntilEOT")
	}
	if !e.HasKeyword(gid, "Flying") {
		t.Fatal("precondition: Gabriel should have Flying")
	}

	driveToStepAll(t, e, 4, 1, state.StepMain1)
	wantUpkeepRegs(t, e, 1, "turn 4 (survives this turn's cleanup)")
	if !e.HasKeyword(gid, "Flying") {
		t.Error("Gabriel lost Flying a turn early")
	}

	driveToStepAll(t, e, 5, 0, state.StepUpkeep)
	wantUpkeepRegs(t, e, 0, "turn 5 upkeep")
	if e.HasKeyword(gid, "Flying") {
		t.Error("Gabriel still has Flying at its controller's next upkeep")
	}
	replayCheck(t, e, cfg)
}

func TestUntilYourNextUpkeepAnimateExpiresAtOwnUpkeep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Xenic Poltergeist"), lookup(t, reg, "Sol Ring")}, nil)
	xenic := moveByName(t, e, 0, "Xenic Poltergeist", state.ZBattlefield)
	ring := moveByName(t, e, 0, "Sol Ring", state.ZBattlefield)
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	for id, want := range map[state.ObjID]state.Zone{xenic: state.ZBattlefield, ring: state.ZBattlefield} {
		if z := e.G.Obj(id).Zone; z != want {
			t.Fatalf("precondition: object %d is in %v, want %v", id, z, want)
		}
	}
	if e.IsCreature(ring) {
		t.Fatal("precondition: Sol Ring is already a creature")
	}
	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, xenic, 0).Index)
	answerKTarget(t, e, ring)
	passUntilStackEmpty(t, e, 50)
	if !e.IsCreature(ring) || e.Power(ring) != 1 || e.Toughness(ring) != 1 {
		t.Fatalf("precondition: Sol Ring is creature=%v %d/%d, want a 1/1 creature",
			e.IsCreature(ring), e.Power(ring), e.Toughness(ring))
	}
	wantUpkeepRegs(t, e, 2, "after animating")
	for _, ce := range upkeepRegs(e) {
		if ce.UntilEOT {
			t.Fatalf("an animation half is UntilEOT: %+v", ce)
		}
	}

	driveToStepAll(t, e, 4, 1, state.StepMain1)
	wantUpkeepRegs(t, e, 2, "turn 4 (survives this turn's cleanup)")
	if !e.IsCreature(ring) || e.Power(ring) != 1 || e.Toughness(ring) != 1 {
		t.Errorf("Sol Ring is creature=%v %d/%d on turn 4, want a 1/1 creature",
			e.IsCreature(ring), e.Power(ring), e.Toughness(ring))
	}

	driveToStepAll(t, e, 5, 0, state.StepUpkeep)
	wantUpkeepRegs(t, e, 0, "turn 5 upkeep")
	if e.IsCreature(ring) {
		t.Error("Sol Ring is still a creature at its animator's next upkeep")
	}
	replayCheck(t, e, cfg)
}
