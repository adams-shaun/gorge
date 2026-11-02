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

// DelayedTriggerDefinedPlayer$ / UpcomingTurn$ (CR 603.7c): a Mode$ Phase
// delayed trigger registered with a defined player is armed only from that
// player's NEXT turn, and with ThisTurn$ True it dies when that turn ends.
// Static$ True ones then resolve off the stack (Forge's isStatic arm).

// resolveDelayedTriggerAbility resolves the first DelayedTrigger ability of
// the source object's face (or its SVar named svar when non-empty) with the
// given targets, the way the stack would.
func resolveDelayedTriggerAbility(t *testing.T, e *Engine, src state.ObjID, ctrl state.PlayerID, svar string, targets ...state.Target) {
	t.Helper()
	o := e.G.Obj(src)
	if o == nil || o.Face() == nil {
		t.Fatalf("source %d has no face", src)
	}
	var sa *cards.SA
	if svar != "" {
		sa = cards.ResolveSVar(o.Face().SVars, svar)
	} else {
		for _, a := range o.Face().Abilities {
			if a.API == "DelayedTrigger" {
				sa = a
				break
			}
		}
	}
	if sa == nil || sa.API != "DelayedTrigger" {
		t.Fatalf("no DelayedTrigger ability on %q (svar %q)", o.Face().Name, svar)
	}
	ctx := effects.NewCtx(src, ctrl, effects.CtxInit{Targets: targets})
	effects.Resolve(e, &ctx, sa)
	e.pending = nil
}

func countMustAttackEffects(e *Engine, src state.ObjID) int {
	n := 0
	for _, ce := range e.continuous {
		if ce.Source == src && ce.Restriction != "" {
			n++
		}
	}
	return n
}

func assertNoDelayedStackObject(t *testing.T, e *Engine, src state.ObjID, from int) {
	t.Helper()
	for _, ev := range e.L.Events[from:] {
		if (ev.Kind == events.TriggerPush || ev.Kind == events.DelayedPush && ev.Text != "static") && ev.Obj == src {
			t.Fatalf("the Static$ True delayed trigger was pushed on the stack: %+v", ev)
		}
		if ev.Kind == events.DecisionAsk && ev.Text == string(decision.KTriggerOrder) {
			t.Fatalf("a static delayed trigger entered the CR 603.3b order choice: %+v", ev)
		}
	}
}

func seat(p state.PlayerID) state.Target { return state.Target{Player: p, IsPlayer: true} }

// TestDelayedDefinedPlayerGideonJuraFiresInOpponentsNextTurn: nothing at
// turn 1; at seat 1's upkeep (turn 2) the MustAttack Effect exists, minted
// off the stack, with the registration consumed.
func TestDelayedDefinedPlayerGideonJuraFiresInOpponentsNextTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	gideon := mustCorpusCard(t, reg, "Gideon Jura")
	e, cfg := tokenReplGame(t, 411, gideon)
	id := moveSeededCard(t, e, 0, gideon, state.ZBattlefield)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("Gideon Jura in zone %v, want battlefield", o.Zone)
	}
	e.Advance()
	mark := len(e.L.Events)
	resolveDelayedTriggerAbility(t, e, id, 0, "", seat(1))
	if hasNote(e, "ignores unread parameter") {
		t.Fatalf("DelayedTrigger still Notes an unread parameter")
	}
	if len(e.G.Delayed) != 1 {
		t.Fatalf("registrations = %d, want 1", len(e.G.Delayed))
	}
	dt := e.G.Delayed[0]
	if dt.DefinedPlayer != 2 || !dt.Static || dt.Phase != state.StepUpkeep {
		t.Fatalf("registration = %+v, want DefinedPlayer 2 (seat 1), Static, Phase Upkeep", dt)
	}

	driveToStepAll(t, e, 1, 0, state.StepEnd)
	if n := countMustAttackEffects(e, id); n != 0 {
		t.Fatalf("MustAttack effects at turn 1 end = %d, want 0: the delay fired in the registering turn", n)
	}
	driveToStepAll(t, e, 2, 1, state.StepUpkeep)
	if n := countMustAttackEffects(e, id); n != 1 {
		t.Fatalf("MustAttack effects at turn 2 upkeep = %d, want 1", n)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("registration not consumed: %+v", e.G.Delayed)
	}
	assertNoDelayedStackObject(t, e, id, mark)
	replayCheck(t, e, cfg)
}

// TestDelayedDefinedPlayerTauntOnCasterWaitsForNextTurn pins "next turn"
// rather than "this turn": Taunt aimed at its own caster does not fire in
// the caster's current turn and fires in the caster's next one (turn 3).
func TestDelayedDefinedPlayerTauntOnCasterWaitsForNextTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	taunt := mustCorpusCard(t, reg, "Taunt")
	e, cfg := tokenReplGame(t, 412, taunt)
	id := moveSeededCard(t, e, 0, taunt, state.ZHand)
	if o := e.G.Obj(id); o.Zone != state.ZHand {
		t.Fatalf("Taunt in zone %v, want hand", o.Zone)
	}
	e.Advance()
	mark := len(e.L.Events)
	resolveDelayedTriggerAbility(t, e, id, 0, "", seat(0))
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].DefinedPlayer != 1 {
		t.Fatalf("registrations = %+v, want one with DefinedPlayer 1 (seat 0)", e.G.Delayed)
	}
	driveToStepAll(t, e, 1, 0, state.StepEnd)
	driveToStepAll(t, e, 2, 1, state.StepUpkeep)
	if n := countMustAttackEffects(e, id); n != 0 {
		t.Fatalf("MustAttack effects at turn 2 = %d, want 0 (the caster's next turn is turn 3)", n)
	}
	driveToStepAll(t, e, 3, 0, state.StepUpkeep)
	if n := countMustAttackEffects(e, id); n != 1 {
		t.Fatalf("MustAttack effects at turn 3 upkeep = %d, want 1", n)
	}
	assertNoDelayedStackObject(t, e, id, mark)
	replayCheck(t, e, cfg)
}

// TestDelayedDefinedPlayerThisTurnFiresAtEndStepOfNextTurn: Suppress's
// DelTrig (DelayedTriggerDefinedPlayer$ Targeted + ThisTurn$ True) fires at
// the end step of the targeted player's next turn, and not before.
func TestDelayedDefinedPlayerThisTurnFiresAtEndStepOfNextTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	suppress := mustCorpusCard(t, reg, "Suppress")
	e, cfg := tokenReplGame(t, 413, suppress)
	id := moveSeededCard(t, e, 0, suppress, state.ZHand)
	e.Advance()
	resolveDelayedTriggerAbility(t, e, id, 0, "DelTrig", seat(1))
	if len(e.G.Delayed) != 1 {
		t.Fatalf("registrations = %d, want 1", len(e.G.Delayed))
	}
	if dt := e.G.Delayed[0]; dt.DefinedPlayer != 2 || !dt.ThisTurnDefined || dt.Phase != state.StepEnd {
		t.Fatalf("registration = %+v, want DefinedPlayer 2, ThisTurnDefined, Phase End", dt)
	}
	pushes := func() int { return countDelayedPushes(e, id) }
	// Turn 1's own end step must not fire it.
	driveToStepAll(t, e, 1, 0, state.StepEnd)
	if n := pushes(); n != 0 {
		t.Fatalf("delayed trigger fired %d time(s) at the registering turn's end step", n)
	}
	driveToStepAll(t, e, 2, 1, state.StepUpkeep)
	if n := pushes(); n != 0 {
		t.Fatalf("delayed trigger fired %d time(s) before seat 1's end step", n)
	}
	driveToStepAll(t, e, 2, 1, state.StepEnd)
	e.Advance()
	if n := pushes(); n != 1 {
		t.Fatalf("delayed trigger pushes at seat 1's end step = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}

const delayedDefinedPlayerFixture = `Name:Delay Probe
ManaCost:1
Types:Artifact
A:AB$ DelayedTrigger | Cost$ 0 | Mode$ Phase | Phase$ End of Turn | DelayedTriggerDefinedPlayer$ You | ThisTurn$ True | IsPresent$ Creature.YouCtrl | PresentZone$ Battlefield | Execute$ TrigNothing
SVar:TrigNothing:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`

// TestDelayedDefinedPlayerWindowPassedUnfiredIsRemoved: a ThisTurn$
// registration whose gate fails at the one end step it may fire in is
// removed (DelayedRemove) once that turn is over, not left pending.
func TestDelayedDefinedPlayerWindowPassedUnfiredIsRemoved(t *testing.T) {
	t.Parallel()
	probe := card(t, delayedDefinedPlayerFixture)
	e, cfg := tokenReplGame(t, 414, probe)
	id := moveSeededCard(t, e, 0, probe, state.ZBattlefield)
	e.Advance()
	resolveDelayedTriggerAbility(t, e, id, 0, "")
	if len(e.G.Delayed) != 1 {
		t.Fatalf("registrations = %d, want 1", len(e.G.Delayed))
	}
	if dt := e.G.Delayed[0]; dt.DefinedPlayer != 1 || !dt.ThisTurnDefined || dt.PresentSpec == "" {
		t.Fatalf("registration = %+v, want DefinedPlayer 1, ThisTurnDefined and a presence gate", dt)
	}
	if n := len(e.G.Zone(state.ZBattlefield, 0)); n != 1 {
		t.Fatalf("seat 0 battlefield = %d permanents, want only the probe (the gate must fail)", n)
	}
	driveToStepAll(t, e, 3, 0, state.StepEnd)
	if len(e.G.Delayed) != 1 {
		t.Fatalf("registration gone before its window ended: %+v", e.G.Delayed)
	}
	driveToStepAll(t, e, 4, 1, state.StepUpkeep)
	if len(e.G.Delayed) != 0 || !hasEventKind(e, events.DelayedRemove) {
		t.Fatalf("expired registration not removed: %+v", e.G.Delayed)
	}
	replayCheck(t, e, cfg)
}

const delayedStaticAskFixture = `Name:Ask Probe
ManaCost:1
Types:Artifact
A:AB$ DelayedTrigger | Cost$ 0 | Mode$ Phase | Phase$ Upkeep | DelayedTriggerDefinedPlayer$ You | Static$ True | Execute$ TrigAsk
SVar:TrigAsk:DB$ GenericChoice | Choices$ ChoiceA,ChoiceB | Defined$ You
SVar:ChoiceA:DB$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ gain 1
SVar:ChoiceB:DB$ GainLife | Defined$ You | LifeAmount$ 2 | SpellDescription$ gain 2
Oracle:x
`

// TestDelayedStaticAskingBodyStaysOnStack: a Static$ True Phase delayed
// trigger whose body asks keeps the stack path (the ask-free gate).
func TestDelayedStaticAskingBodyStaysOnStack(t *testing.T) {
	t.Parallel()
	probe := card(t, delayedStaticAskFixture)
	e, cfg := tokenReplGame(t, 415, probe)
	id := moveSeededCard(t, e, 0, probe, state.ZBattlefield)
	e.Advance()
	resolveDelayedTriggerAbility(t, e, id, 0, "")
	if len(e.G.Delayed) != 1 || !e.G.Delayed[0].Static || hasNote(e, "ignores unread parameter") {
		t.Fatalf("registration = %+v, want one Static registration and no unread Note", e.G.Delayed)
	}
	driveToStepAll(t, e, 1, 0, state.StepEnd)
	// Turn 3 upkeep: the choice ask is pending (or already answered).
	for i := 0; i < 4000 && !hasPushOf(e, id); i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e)
		case decision.KAttackers, decision.KBlockers:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: nil}); err != nil {
				t.Fatalf("submit %s: %v", d.Kind, err)
			}
		default:
			if len(d.Options) == 0 {
				t.Fatalf("decision with no options: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err != nil {
				t.Fatalf("submit %s: %v", d.Kind, err)
			}
		}
	}
	if !hasPushOf(e, id) {
		t.Fatal("the asking Static$ True delayed trigger never reached the stack")
	}
	if e.G.Turn != 3 {
		t.Fatalf("pushed at turn %d, want 3 (the caster's next turn)", e.G.Turn)
	}
	replayCheck(t, e, cfg)
}

// countDelayedPushes counts non-static DelayedPush events for src (the
// event's Obj is the registering source).
func countDelayedPushes(e *Engine, src state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedPush && ev.Text != "static" && ev.Obj == src {
			n++
		}
	}
	return n
}

func hasPushOf(e *Engine, src state.ObjID) bool { return countDelayedPushes(e, src) > 0 }
