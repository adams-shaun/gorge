package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The trigger-body Cost$ ticket (trigcost1). Forge's `Cost$ <cost>` on a
// trigger body is the "you may pay <cost>. If you do, ..." idiom; before this
// ticket only the Untap / ImmediateTrigger / Draw / dyn-tap / CopySpellAbility
// shapes entered the triggered-cost pay/decline window, so e.g. Kalastria
// Highborn's `Cost$ B` executed its drain for FREE (no ask, no payment). The
// fix widens both gate sites (resolveTop's mandatory arm and resumeResolution's
// "optional" arm) through the shared triggerBodyNeedsCostWindow helper; the
// `Mandatory`-prefixed family is deliberately carved out (its real payment is
// a non-mana settle rules does not yet run, and a decline-only ask would be
// the WORSE regression).
//
// All fixtures are real compiled corpus cards; none is in any legacy golden
// deck, so TestHeads does not depend on these cards' behaviour changing.

// triggerCostWindowAsk drives the engine until a non-priority decision is
// pending and returns the pay ("trigger_cost_pay") and decline
// ("trigger_cost_decline") option indices within it.
func triggerCostWindowAsk(t *testing.T, e *Engine) (pay, decline int) {
	pay, decline = -1, -1
	t.Helper()
	d := passUntilNonPriority(t, e, 40)
	for _, o := range d.Options {
		switch o.Kind {
		case "trigger_cost_pay":
			pay = o.Index
		case "trigger_cost_decline":
			decline = o.Index
		}
	}
	if decline < 0 {
		t.Fatalf("the trigger-cost window never opened (pending %+v)", d)
	}
	return pay, decline
}

// answerPlayerTargetAsk drives until the pending target ask and submits the
// option naming the given seat (the Kalastria ask's ValidTgts$ Player options).
func answerPlayerTargetAsk(t *testing.T, e *Engine, seat state.PlayerID) {
	t.Helper()
	d := passUntilNonPriority(t, e, 20)
	if d.Kind != decision.KTarget {
		t.Fatalf("expected a target ask, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Player == seat {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("target ask has no option for seat %d: %+v", seat, d.Options)
}

// killOnBattlefield moves a battlefield permanent to the graveyard (the death
// the ChangesZone triggers key on) and re-opens priority so the trigger drains.
func killOnBattlefield(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
}

// TestKalastriaTriggerCostPayChargesAndDrains pins the widened optional arm on
// the real corpus SA: answering PAY charges the {B} from the pool and the
// drain resolves (target player loses 2, controller gains 2).
func TestKalastriaTriggerCostPayChargesAndDrains(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Kalastria Highborn")
	id := searchMoveByName(t, e, "Kalastria Highborn", state.ZBattlefield)
	addMana(t, e, 0, "B")
	oppLife, myLife := e.G.Players[1].Life, e.G.Players[0].Life

	killOnBattlefield(t, e, id)
	answerPlayerTargetAsk(t, e, 1) // target the opponent

	// The optional ask: "Apply this triggered ability's effect?" -- yes.
	d := passUntilNonPriority(t, e, 20)
	if d.Kind != decision.KTriggerOptional || len(d.Options) == 0 || d.Options[0].Kind != "yes" {
		t.Fatalf("expected the trigger_optional yes ask, got %+v", d)
	}
	submitChoices(t, e, 0)

	pay, _ := triggerCostWindowAsk(t, e)
	if pay < 0 {
		t.Fatalf("the Cost$ B body was not offered as payable: %+v", e.Pending())
	}
	poolBefore := e.G.Players[0].Pool[state.MB]
	submitChoices(t, e, pay)
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Players[0].Pool[state.MB]; got != poolBefore-1 {
		t.Fatalf("pool {B} = %d after paying, want %d (the cost must come OFF the pool)", got, poolBefore-1)
	}
	if got := e.G.Players[1].Life; got != oppLife-2 {
		t.Fatalf("opponent life = %d, want %d (the paid drain)", got, oppLife-2)
	}
	if got := e.G.Players[0].Life; got != myLife+2 {
		t.Fatalf("my life = %d, want %d (the paid gain)", got, myLife+2)
	}
}

// TestKalastriaTriggerCostDeclineChangesNothing pins the decline arm: a
// declined {B} body does NOT drain and does NOT gain.
func TestKalastriaTriggerCostDeclineChangesNothing(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Kalastria Highborn")
	id := searchMoveByName(t, e, "Kalastria Highborn", state.ZBattlefield)
	addMana(t, e, 0, "B")
	oppLife, myLife := e.G.Players[1].Life, e.G.Players[0].Life

	killOnBattlefield(t, e, id)
	answerPlayerTargetAsk(t, e, 1)

	d := passUntilNonPriority(t, e, 20)
	if d.Kind != decision.KTriggerOptional {
		t.Fatalf("expected the trigger_optional ask, got %+v", d)
	}
	submitChoices(t, e, 0)

	_, decline := triggerCostWindowAsk(t, e)
	mark := len(e.L.Events)
	submitChoices(t, e, decline)
	passUntilStackEmpty(t, e, 20)

	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.LifeChange {
			t.Fatalf("a declined trigger cost still changed life: %+v", ev)
		}
	}
	if got := e.G.Players[1].Life; got != oppLife {
		t.Fatalf("opponent life = %d after a decline, want unchanged %d", got, oppLife)
	}
	if got := e.G.Players[0].Life; got != myLife {
		t.Fatalf("my life = %d after a decline, want unchanged %d", got, myLife)
	}
	if got := e.G.Players[0].Pool[state.MB]; got != 1 {
		t.Fatalf("pool {B} = %d after a decline, want the untouched 1", got)
	}
}

// elendaEndStepCostAsk moves Elenda and Azor onto the battlefield, drives to
// the end step and returns the pending trigger-cost window ask for the
// `Cost$ PayLife<4>` body (no OptionalDecider$, so this exercises the widened
// MANDATORY arm in resolveTop).
func elendaEndStepCostAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	searchMoveByName(t, e, "Elenda and Azor", state.ZBattlefield)
	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	d := passUntilNonPriority(t, e, 40)
	if d == nil {
		t.Fatal("no decision pending at the end step")
	}
	return d
}

// TestElendaAndAzorEndStepPayLifeCharges pins the mandatory arm: Elenda and
// Azor's end-step body (`AB$ Token | Cost$ PayLife<4> | TokenAmount$ Y`) is
// "you may pay 4 life. If you do, ..." -- paying charges 4 life.
func TestElendaAndAzorEndStepPayLifeCharges(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Elenda and Azor")
	myLife := e.G.Players[0].Life

	d := elendaEndStepCostAsk(t, e)
	pay, _ := triggerCostWindowAskDecision(t, d)
	if pay < 0 {
		t.Fatalf("the PayLife<4> body was not offered as payable: %+v", d.Options)
	}
	submitChoices(t, e, pay)
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Players[0].Life; got != myLife-4 {
		t.Fatalf("life = %d after paying the end-step cost, want %d", got, myLife-4)
	}
}

// TestElendaAndAzorEndStepPayLifeDeclineChargesNone is the decline half.
func TestElendaAndAzorEndStepPayLifeDeclineChargesNone(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Elenda and Azor")
	myLife := e.G.Players[0].Life

	d := elendaEndStepCostAsk(t, e)
	_, decline := triggerCostWindowAskDecision(t, d)
	submitChoices(t, e, decline)
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Players[0].Life; got != myLife {
		t.Fatalf("life = %d after declining, want unchanged %d", got, myLife)
	}
}

// triggerCostWindowAskDecision is the decision-taking variant of
// triggerCostWindowAsk (the ask is already pending).
func triggerCostWindowAskDecision(t *testing.T, d *decision.Decision) (pay, decline int) {
	pay, decline = -1, -1
	t.Helper()
	for _, o := range d.Options {
		switch o.Kind {
		case "trigger_cost_pay":
			pay = o.Index
		case "trigger_cost_decline":
			decline = o.Index
		}
	}
	if decline < 0 {
		t.Fatalf("no trigger-cost window ask (pending %+v)", d)
	}
	return pay, decline
}

// TestPromiseOfBunreiMandatorySacrificeChargesAndTokens replaces the old
// TestMandatoryCostBodyKeepsCurrentFreeExecution, which pinned the defect: a
// `Cost$ Mandatory Sac<1/CARDNAME>` body used to execute for FREE, leaving
// Promise of Bunrei on the battlefield. trigmand1 settles the mandatory cost
// for real -- the source is sacrificed (a real events.Sacrifice), and only
// then does the body run and create its four 1/1 Spirits. No pay/decline
// election is ever posed for a mandatory cost.
func TestPromiseOfBunreiMandatorySacrificeChargesAndTokens(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Promise of Bunrei")
	promise := searchMoveByName(t, e, "Promise of Bunrei", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)

	killOnBattlefield(t, e, bear)
	passUntilStackEmpty(t, e, 20)

	tokens := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.TokenCreate && ev.Text == "c_1_1_spirit" {
			tokens++
		}
	}
	if tokens != 4 {
		t.Fatalf("token_create events for c_1_1_spirit = %d, want 4 (the paid body must run)", tokens)
	}
	if o := e.G.Obj(promise); o == nil || o.Zone == state.ZBattlefield {
		t.Fatalf("Promise of Bunrei is still on the battlefield; the Mandatory Sac cost must be charged")
	}
	if o := e.G.Obj(promise); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Promise of Bunrei zone = %v, want the graveyard after being sacrificed", o)
	}
	sawSac := false
	for _, ev := range e.L.Events {
		if events.IsSacrifice(ev) && ev.Obj == promise {
			sawSac = true
		}
	}
	if !sawSac {
		t.Fatalf("no events.Sacrifice for Promise of Bunrei; the cost was never settled")
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && (ev.Text == "trigger_cost_pay" || ev.Text == "trigger_cost_decline") {
			t.Fatalf("a mandatory cost posed a pay/decline election: %+v", ev)
		}
	}
}

// TestMandatorySacUnpayableBodyDoesNotRun is a SYNTHETIC fixture (built inline,
// no corpus script committed): a `Cost$ Mandatory Sac<1/Artifact>` trigger body
// with zero artifacts on the battlefield cannot pay its cost, so the body must
// be skipped -- the draw must not happen. This is the mandatory semantics'
// second half: if the cost cannot be paid, the effect does not run.
func TestMandatorySacUnpayableBodyDoesNotRun(t *testing.T) {
	const script = "Name:Ingot Eater\nManaCost:1 R\nTypes:Creature Human Warrior\nPT:2/2\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigSac | TriggerDescription$ When CARDNAME enters, sacrifice an artifact. If you do, draw a card.\n" +
		"SVar:TrigSac:AB$ Draw | Cost$ Mandatory Sac<1/Artifact> | NumCards$ 1\n" +
		"Oracle:x\n"
	e := handEngine(t)
	src := e.G.AddObject(card(t, script), 0)
	src.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{src.ID})
	mark := len(e.L.Events)
	// The ETB event fires the ChangesZone trigger; the board holds only the
	// non-artifact source, so Sac<1/Artifact> has zero eligible candidates.
	e.emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	e.resolveTop()
	if d := e.Pending(); d != nil {
		t.Fatalf("an unpayable mandatory cost posed a decision: %+v", d)
	}
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Draw {
			t.Fatalf("the body ran even though the mandatory sacrifice was unpayable: %+v", ev)
		}
	}
	if o := e.G.Obj(src.ID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the source must not be sacrificed when the cost is unpayable")
	}
}

// TestDalekIntensiveCareMandatoryExileIsAChoice covers the choice-bearing
// mandatory component on Dalek Intensive Care's exact cost shape
// (`Exile<1/Creature.nonDalek/non-Dalek creature>`): with two eligible
// non-Dalek creatures the exile is a real KChoose (Min=Max=1), and the picked
// creature moves to exile before the body runs -- no pay/decline election is
// posed. The fixture carries Dalek's real cost spec verbatim; its trigger is
// an ETB rather than the card's own upkeep line because forge's command-zone
// TriggerZones$ is not scanned by this build's trigger walk (see the report's
// ## Issues), so the real card cannot fire in a test. The cost settle under
// test is exactly the real card's.
func TestDalekIntensiveCareMandatoryExileIsAChoice(t *testing.T) {
	const dalekCostScript = "Name:Dalek Intensive Care\nTypes:Plane Dalek\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigExile\n" +
		"SVar:TrigExile:AB$ Draw | Cost$ Mandatory Exile<1/Creature.nonDalek/non-Dalek creature> | NumCards$ 1\n" +
		"Oracle:x\n"
	const bearScript = "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e := handEngine(t)
	src := e.G.AddObject(card(t, dalekCostScript), 0)
	src.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{src.ID})
	bearA := onBoard(t, e, 0, bearScript)
	bearB := onBoard(t, e, 0, bearScript)

	// The ETB fires the ChangesZone trigger; the window settles the mandatory
	// Exile, which has two eligible non-Dalek creatures and therefore a real
	// choice.
	e.emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the mandatory exile KChoose, got %+v", d)
	}
	if d.Min != 1 || d.Max != 1 {
		t.Fatalf("mandatory exile ask bounds = %d..%d, want 1..1", d.Min, d.Max)
	}
	if len(d.Options) != 2 {
		t.Fatalf("mandatory exile ask options = %d, want the 2 eligible creatures", len(d.Options))
	}
	pick := -1
	for _, o := range d.Options {
		if o.Kind != "exile_cost" {
			t.Fatalf("mandatory exile option kind = %q, want exile_cost", o.Kind)
		}
		if o.Obj == bearA {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("the exile ask does not offer the first eligible bear: %+v", d.Options)
	}
	mark := len(e.L.Events)
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(bearA); o == nil || o.Zone != state.ZExile {
		t.Fatalf("the picked creature zone = %v, want exile", o)
	}
	if o := e.G.Obj(bearB); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the unpicked creature must stay on the battlefield")
	}
	if o := e.G.Obj(src.ID); o == nil || o.Zone == state.ZExile {
		t.Fatalf("the non-creature source must not be exiled by Creature.nonDalek")
	}
	sawDraw := false
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Draw {
			sawDraw = true
		}
	}
	if !sawDraw {
		t.Fatal("the body did not run after the mandatory exile was paid")
	}
}

// Kuldotha Flamefiend's ETB body carries `Cost$ Sac<1/Artifact>` (a real
// non-mana component payMana cannot charge). The optional yes reaches the
// window, which must offer DECLINE ONLY -- no trigger_cost_pay option, never
// a free execution -- and the decline runs neither the sacrifice nor the
// damage.
func TestUnpriceableOptionalBodyDeclineOnly(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Kuldotha Flamefiend")
	id := searchMoveByName(t, e, "Kuldotha Flamefiend", state.ZBattlefield)

	// The body's target ask (ValidTgts$ Any, TargetMin$ 0): answer empty.
	d := passUntilNonPriority(t, e, 20)
	if d.Kind != decision.KTarget {
		t.Fatalf("expected Flamefiend's target ask, got %+v", d)
	}
	submitChoices(t, e)

	d = passUntilNonPriority(t, e, 20)
	if d.Kind != decision.KTriggerOptional {
		t.Fatalf("expected the trigger_optional ask, got %+v", d)
	}
	submitChoices(t, e, 0) // yes, attempt the effect

	pay, decline := triggerCostWindowAsk(t, e)
	if pay >= 0 {
		t.Fatalf("the unpriceable Sac<1/Artifact> cost was offered as payable: %+v", e.Pending().Options)
	}
	mark := len(e.L.Events)
	submitChoices(t, e, decline)
	passUntilStackEmpty(t, e, 20)

	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Damage {
			t.Fatalf("a declined unpriceable body still dealt damage: %+v", ev)
		}
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Flamefiend left the battlefield on a declined cost")
	}
	for _, z := range e.G.Zone(state.ZGraveyard, 0) {
		if o := e.G.Obj(z); o != nil && o.ID != id && o.Face() != nil && o.Face().Types[0] == "Artifact" {
			t.Fatalf("an artifact was sacrificed on a declined cost")
		}
	}
}
