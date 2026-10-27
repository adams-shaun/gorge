package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const castSAZapSrc = "Name:Little Zap\nManaCost:R\nTypes:Instant\nA:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:x\n"

// castSAResponseCast casts the named creature (mode "optionalcost" collects
// Colossal Dreadmaw as evidence), then casts Little Zap in response so the
// zap resolves first, and drains the stack answering target asks with want
// (the foe's Bear when want is zero). It returns the engine, the creature
// spell's id and the foe's Bear.
func castSAResponseCast(t *testing.T, seed uint64, name, pool, mode string, selfTarget bool) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg, _ := altCostEngine(t, seed, []string{name, "Colossal Dreadmaw"}, []string{castSAZapSrc}, []string{altBearSrc})
	spell := findCardObj(t, e, 0, name, state.ZHand)
	zap := findCardObj(t, e, 0, "Little Zap", state.ZHand)
	colossal := evidenceToGraveyard(t, e)
	foeBear := findCardObj(t, e, 1, "Bear", state.ZBattlefield)
	addMana(t, e, 0, pool)
	submitChoices(t, e, castModeOption(t, e, spell, mode))
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		choosePendingObject(t, e, colossal)
	}
	if mode == "optionalcost" {
		if o := e.G.Obj(colossal); o == nil || o.Zone != state.ZExile {
			t.Fatal("precondition: the evidence card was not exiled")
		}
	}
	if len(e.G.Stack) != 1 || e.G.Stack[0] != spell {
		t.Fatalf("precondition: stack = %v, want just the creature spell", e.G.Stack)
	}
	submitChoices(t, e, castModeOption(t, e, zap, ""))
	if len(e.G.Stack) != 2 || e.G.Stack[0] != spell {
		t.Fatalf("precondition: stack = %v, want the creature spell then the response", e.G.Stack)
	}
	want := foeBear
	if selfTarget {
		want = spell
	}
	drainAnswering(t, e, want)
	return e, cfg, spell, foeBear
}

// TestCrimestopperSpriteStunSurvivesResponse: CR 601.2h -- the evidence the
// Sprite's cast collected is a fact of that cast, so a spell cast in response
// does not erase the stun counter.
func TestCrimestopperSpriteStunSurvivesResponse(t *testing.T) {
	t.Parallel()
	e, cfg, _, bear := castSAResponseCast(t, 733, "Crimestopper Sprite", "CCUR", "optionalcost", false)
	o := e.G.Obj(bear)
	if o == nil || !o.Tapped {
		t.Fatalf("precondition: the foe's Bear is not tapped: %+v", o)
	}
	if got := o.Counter("STUN"); got != 1 {
		t.Fatalf("stun counters = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// TestVituGhaziInspectorGateSurvivesResponse: the intervening-if of the ETB
// holds at fire time and on resolution (CR 603.4) despite the response.
func TestVituGhaziInspectorGateSurvivesResponse(t *testing.T) {
	t.Parallel()
	e, cfg, inspector, _ := castSAResponseCast(t, 733, "Vitu-Ghazi Inspector", "CGR", "optionalcost", true)
	if got := e.G.Players[0].Life; got != 23 {
		t.Fatalf("life = %d, want 23 (Zap 1 + the evidence-gated 2)", got)
	}
	if got := e.G.Obj(inspector).Counter("P1P1"); got != 1 {
		t.Fatalf("+1/+1 counters = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// TestCrimestopperSpriteNoEvidenceNoStun: the unpaid cast taps without a stun
// counter, with the same response.
func TestCrimestopperSpriteNoEvidenceNoStun(t *testing.T) {
	t.Parallel()
	e, cfg, _, bear := castSAResponseCast(t, 733, "Crimestopper Sprite", "CCUR", "", false)
	o := e.G.Obj(bear)
	if o == nil || !o.Tapped {
		t.Fatalf("precondition: the foe's Bear is not tapped: %+v", o)
	}
	if got := o.Counter("STUN"); got != 0 {
		t.Fatalf("stun counters = %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// TestVituGhaziInspectorBlinkedLosesEvidence: CR 400.7 -- after the paid cast
// resolves, a blink makes a new object that was never cast, so its ETB gate
// reads no evidence.
func TestVituGhaziInspectorBlinkedLosesEvidence(t *testing.T) {
	t.Parallel()
	e, cfg, inspector := vituGhaziCast(t, 733, "optionalcost")
	if got := e.G.Players[0].Life; got != 22 {
		t.Fatalf("precondition: life = %d, want 22 after the paid cast", got)
	}
	if !e.WasCast(inspector) {
		t.Fatal("precondition: the resolved Inspector reads as cast")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: inspector, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: inspector, From: state.ZExile, To: state.ZBattlefield})
	if e.WasCast(inspector) {
		t.Fatal("the re-entered Inspector reads as cast")
	}
	e.pending = nil
	e.priorityRound()
	drainAnswering(t, e, inspector)
	if o := e.G.Obj(inspector); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Inspector not back on the battlefield: %+v", o)
	}
	if got := e.G.Players[0].Life; got != 22 {
		t.Fatalf("life = %d, want 22 (the re-entry ETB gate must fail)", got)
	}
	if got := e.G.Obj(inspector).Counter("P1P1"); got != 0 {
		t.Fatalf("+1/+1 counters = %d, want 0 on the new object", got)
	}
	replayCheck(t, e, cfg)
}
