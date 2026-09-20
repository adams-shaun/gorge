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

// TestMandatoryCostBodyKeepsCurrentFreeExecution pins the carve-out: a
// `Cost$ Mandatory Sac<1/CARDNAME>` body (Promise of Bunrei) still executes
// its effect and is NOT turned into a decline-only ask -- the cost itself
// stays uncharged on the current path (the real mandatory payment is a
// follow-up ticket). Over-widening the gate onto this family would make the
// body do NOTHING, which is the regression this test catches.
func TestMandatoryCostBodyKeepsCurrentFreeExecution(t *testing.T) {
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
		t.Fatalf("token_create events for c_1_1_spirit = %d, want 4 (the mandatory body still runs)", tokens)
	}
	if o := e.G.Obj(promise); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Promise of Bunrei left the battlefield; the carve-out must keep today's free execution")
	}
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending after the mandatory trigger resolved")
	}
	for _, o := range d.Options {
		if o.Kind == "trigger_cost_pay" || o.Kind == "trigger_cost_decline" {
			t.Fatalf("the Mandatory body was routed into the pay window: %+v", d.Options)
		}
	}
}

// TestUnpriceableOptionalBodyDeclineOnly pins the window's unpriceable split:
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
