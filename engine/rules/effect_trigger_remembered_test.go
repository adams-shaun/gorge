package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// An Effect's own `IsRemembered` trigger reads the Effect's RememberObjects$
// capture (CR 611.2a / 113.6), not the resolving spell's memory (empty for a
// plain `SP$ Effect | ValidTgts$ ... | RememberObjects$ Targeted` spell).
const (
	rememberInsightSrc = "Name:Insight Fixture\nManaCost:0\nTypes:Instant\n" +
		"A:SP$ Effect | ValidTgts$ Permanent.YouCtrl | Triggers$ TrigTap | RememberObjects$ Targeted | SpellDescription$ x\n" +
		"SVar:TrigTap:Mode$ Taps | ValidCard$ Card.IsRemembered | Execute$ TrigGain\n" +
		"SVar:TrigGain:DB$ GainLife | LifeAmount$ 3\nOracle:x\n"
	rememberTapperSrc = "Name:Tapper Fixture\nTypes:Artifact\n" +
		"A:AB$ GainLife | Cost$ T | LifeAmount$ 0\nOracle:x\n"
)

// rememberInsightBoard casts the Insight on the first tapper and drains the
// stack. It returns the engine, config and both tappers (the second one is
// never targeted).
func rememberInsightBoard(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	insight := card(t, rememberInsightSrc)
	tapA, tapB := card(t, rememberTapperSrc), card(t, rememberTapperSrc)
	e, cfg := tokenReplGame(t, 5, insight, tapA, tapB)
	a := moveSeededCard(t, e, 0, tapA, state.ZBattlefield)
	b := moveSeededCard(t, e, 0, tapB, state.ZBattlefield)
	if b == 0 || a == b {
		t.Fatalf("precondition: two distinct tappers on the battlefield (a=%d b=%d)", a, b)
	}
	spell := moveSeededCard(t, e, 0, insight, state.ZHand)
	e.pending = nil
	addMana(t, e, 0, "")
	submitChoices(t, e, castOption(t, e, spell))
	submitChoices(t, e, targetOptionIdx(t, e, a))
	passUntilStackEmpty(t, e, 20)
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: Insight did not resolve")
	}
	return e, cfg, a, b
}

func tapForLife(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, id, 0).Index)
	passUntilStackEmpty(t, e, 20)
}

func TestEffectTriggerIsRememberedReadsEffectCapture(t *testing.T) {
	e, cfg, a, _ := rememberInsightBoard(t)
	if e.G.Players[0].Life != 20 {
		t.Fatalf("precondition: life %d before the tap, want 20", e.G.Players[0].Life)
	}
	tapForLife(t, e, a)
	if e.G.Players[0].Life != 23 {
		t.Fatalf("life after the remembered permanent tapped = %d, want 23", e.G.Players[0].Life)
	}
	replayCheck(t, e, cfg)
}

func TestEffectTriggerIsRememberedIgnoresOtherObjects(t *testing.T) {
	e, cfg, a, b := rememberInsightBoard(t)
	tapForLife(t, e, b)
	if e.G.Players[0].Life != 20 {
		t.Fatalf("life after an UN-remembered permanent tapped = %d, want 20", e.G.Players[0].Life)
	}
	if !e.G.Obj(b).Tapped || e.G.Obj(a).Tapped {
		t.Fatalf("precondition: only the untargeted tapper should be tapped (a=%v b=%v)", e.G.Obj(a).Tapped, e.G.Obj(b).Tapped)
	}
	replayCheck(t, e, cfg)
}

const (
	howlerOutletSrc = "Name:Discard Outlet Fixture\nTypes:Artifact\n" +
		"A:AB$ GainLife | Cost$ Discard<1/Card> | LifeAmount$ 0 | SpellDescription$ x\nOracle:x\n"
	howlerBearSrc = "Name:Howler Bear Fixture\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nK:Haste\nOracle:x\n"
)

// Captain Howler, Sea Scourge's DBEffect (`RememberObjects$ Targeted |
// Triggers$ TrigDamage`, `ValidSource$ Creature.IsRemembered`) draws when the
// pumped creature deals combat damage to a player: the Effect trigger reads
// the Effect's own capture, not Howler's resolving memory.
func TestCaptainHowlerDrawsOnCombatDamage(t *testing.T) {
	howler := lookup(t, testutil.CorpusRegistry(t), "Captain Howler, Sea Scourge")
	outlet, bear := card(t, howlerOutletSrc), card(t, howlerBearSrc)
	e, cfg := tokenReplGame(t, 7, howler, outlet, bear)
	moveSeededCard(t, e, 0, howler, state.ZBattlefield)
	out := moveSeededCard(t, e, 0, outlet, state.ZBattlefield)
	b := moveSeededCard(t, e, 0, bear, state.ZBattlefield)
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, out, 0).Index)
	if !answerIfDiscard(t, e) {
		t.Fatalf("precondition: the outlet's discard cost was not asked: %+v", e.Pending())
	}
	d := passUntilNonPriority(t, e, 30)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("precondition: Howler's discard trigger did not ask for a target: %+v", d)
	}
	submitChoices(t, e, targetOptionIdx(t, e, b))
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(b); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the bear left the battlefield: %+v", o)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	life := e.G.Players[1].Life

	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, b)
	for i := 0; i < 40 && e.G.Players[1].Life == life; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision while attacking: %+v", d)
		}
		submitPass(t, e)
	}
	if e.G.Players[1].Life >= life {
		t.Fatalf("precondition: the bear dealt no combat damage (life %d)", e.G.Players[1].Life)
	}
	passUntilStackEmpty(t, e, 30)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand after the pumped creature connected = %d, want %d (one card drawn)", got, hand+1)
	}
	replayCheck(t, e, cfg)
}

const (
	eaglesBearASrc = "Name:Eagle Bear A\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
	eaglesBearBSrc = "Name:Eagle Bear B\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
)

// The Eagles Are Coming!'s DBDelayedTrigger Effect reads `RememberObjects$
// Remembered` (the spell's own memory), so the corrected registration set is
// the one it always had: both returned creatures, and the next-upkeep body
// still fires. (The token COUNT that body derives is a separate defect: every
// delayed fire seeds Ctx.Captured with this same set, so Remembered$Amount
// excludes it and reads 0 -- filed as a ticket, not asserted here.)
func TestEaglesAreComingRegistrationCaptureUnchanged(t *testing.T) {
	eagles := lookup(t, testutil.CorpusRegistry(t), "The Eagles Are Coming!")
	bearA, bearB := card(t, eaglesBearASrc), card(t, eaglesBearBSrc)
	e, cfg := tokenReplGame(t, 11, eagles, bearA, bearB)
	a := moveSeededCard(t, e, 0, bearA, state.ZBattlefield)
	b := moveSeededCard(t, e, 0, bearB, state.ZBattlefield)
	spell := moveSeededCard(t, e, 0, eagles, state.ZHand)
	addMana(t, e, 0, "WWWWCC")
	submitChoices(t, e, castModeOption(t, e, spell, "kicked"))
	d := passUntilNonPriority(t, e, 30)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("precondition: the kicked cast did not ask for targets: %+v", d)
	}
	answerTargetAsk(t, e, []state.ObjID{a, b})
	passUntilStackEmpty(t, e, 30)
	for _, id := range []state.ObjID{a, b} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
			t.Fatalf("precondition: creature %d was not returned to hand: %+v", id, o)
		}
	}
	driveToStep(t, e, 2, 1, state.StepDraw)
	passUntilStackEmpty(t, e, 30)
	registered, pushed := 0, 0
	for _, ev := range e.L.Events {
		switch {
		case ev.Kind == events.DelayedRegister && ev.Counter == "TrigToken":
			registered++
			if len(ev.IDs) != 2 || ev.IDs[0] != a || ev.IDs[1] != b {
				t.Fatalf("registration capture = %v, want the two returned creatures [%d %d]", ev.IDs, a, b)
			}
		case ev.Kind == events.DelayedPush && ev.Counter == "TrigToken":
			pushed++
		}
	}
	if registered != 1 || pushed != 1 {
		t.Fatalf("Eagles registrations = %d, next-upkeep firings = %d, want 1 and 1", registered, pushed)
	}
	replayCheck(t, e, cfg)
}
