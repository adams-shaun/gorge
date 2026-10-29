package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSunderingEruptionNonFlyersCantBlock pins the reported card end to end:
// its spell chain's Effect SA delivers a Continuous body whose only grant is
// `AddHiddenKeyword$ CARDNAME can't block.`, so after it resolves a creature
// without flying is not offered as a blocker this turn, a flyer still is, and
// the restriction is gone on the next turn.
func TestSunderingEruptionNonFlyersCantBlock(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Sundering Eruption"), card(t, hiddenTestAttacker)},
		[]*cards.Card{card(t, hiddenTestNonflyer), card(t, hiddenTestFlyer)})
	ensureInHand(t, e, 0, "Sundering Eruption")
	attacker := moveByName(t, e, 0, "Test Attacker", state.ZBattlefield)
	nonflyer := moveByName(t, e, 1, "Test Nonflyer", state.ZBattlefield)
	flyer := moveByName(t, e, 1, "Test Flyer", state.ZBattlefield)
	land := moveByName(t, e, 1, "Mountain", state.ZBattlefield)
	if e.G.Obj(land).Zone != state.ZBattlefield {
		t.Fatal("precondition: target land not on the battlefield")
	}

	addMana(t, e, 0, "CCR")
	castCardNow(t, e, "Sundering Eruption")
	// Resort through the real card chain: answer the target ask, decline the
	// optional basic-land search, and pass priority until the stack empties.
	answerTargetAs(t, e, land)
	drainSunderingEruption(t, e)

	if e.G.Obj(nonflyer).Zone != state.ZBattlefield || e.G.Obj(flyer).Zone != state.ZBattlefield {
		t.Fatal("precondition: a test creature left the battlefield during resolution")
	}
	if e.G.Obj(land).Zone == state.ZBattlefield {
		t.Fatal("precondition: the targeted land was not destroyed, so the chain did not resolve")
	}
	declareAttackersOnly(t, e, 0, 1, attacker)
	d := askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("no blockers decision after Sundering Eruption resolved")
	}
	if findBlockOption(d, nonflyer, attacker) != nil {
		t.Fatalf("nonflyer offered as a blocker despite Sundering Eruption: %+v", d.Options)
	}
	if findBlockOption(d, flyer, attacker) == nil {
		t.Fatalf("flyer not offered as a blocker: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{}}); err != nil {
		t.Fatalf("submit empty blockers: %v", err)
	}

	driveToStepAny(t, e, 3, 0, state.StepMain1)
	declareAttackersOnly(t, e, 0, 1, attacker)
	d = askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("no blockers decision on the following turn")
	}
	if findBlockOption(d, nonflyer, attacker) == nil {
		t.Fatalf("nonflyer still blocked on the next turn (the grant did not expire): %+v", d.Options)
	}
	if findBlockOption(d, flyer, attacker) == nil {
		t.Fatalf("flyer not offered on the next turn: %+v", d.Options)
	}
}

// answerTargetAs answers the pending target ask with obj, if one is posed.
// Sundering Eruption's "Destroy target land" has a single legal target here,
// so the engine may auto-target it; only a posed ask needs an answer.
func answerTargetAs(t *testing.T, e *Engine, obj state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		return
	}
	for _, o := range d.Options {
		if o.Obj == obj {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
				t.Fatalf("submit target: %v", err)
			}
			return
		}
	}
	t.Fatalf("no target option for obj %d: %+v", obj, d.Options)
}

// drainSunderingEruption passes priority to an empty stack, declining any
// optional choice (the destroyed land's controller's search) on the way.
func drainSunderingEruption(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40 && !e.G.Over && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining Sundering Eruption (stack depth %d)", len(e.G.Stack))
		}
		switch d.Kind {
		case decision.KPriority:
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
				t.Fatalf("submit pass: %v", err)
			}
		case decision.KChoose:
			// The destroyed land's controller is asked to proceed with the
			// optional basic-land search; answer No (the Min 1 confirmation).
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "no" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("search confirmation with no No option: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("submit optional decline: %v (%+v)", err, d)
			}
		default:
			t.Fatalf("unexpected decision %+v while draining Sundering Eruption", d)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack did not empty: %v", e.G.Stack)
	}
}

// TestParamCensusCreditsEffectDeliveredHiddenKeyword is the regression half:
// the census must credit the key its own StaticAbilities$ body carries as read
// by effects' Effect-delivery registration path, not report it as the printed
// static walk's unread gap. Preconditions assert the measured card really
// carries the param and the reader really contributes it, so an unrelated
// walk change cannot make this test pass vacuously.
func TestParamCensusCreditsEffectDeliveredHiddenKeyword(t *testing.T) {
	t.Parallel()
	_, d := measureParamCensus(t, nil)
	if !d.effectStat["Continuous"]["AddHiddenKeyword"] {
		t.Fatal("precondition: the Effect-delivery static reader contributes no AddHiddenKeyword read")
	}
	reg := testutil.CorpusRegistry(t)
	c := lookup(t, reg, "Sundering Eruption")
	carried := false
	for _, f := range c.Faces {
		f.EachRawEffectChild(func(child cards.EffectChild) {
			if child.Static != nil && child.Static.Params["AddHiddenKeyword"] != "" {
				carried = true
			}
		})
	}
	if !carried {
		t.Fatal("precondition: Sundering Eruption has no Effect-delivered AddHiddenKeyword static")
	}
	for _, label := range cardCensusLabels(c, d, nil) {
		if label == "param:stat:Continuous.AddHiddenKeyword" {
			t.Fatalf("census still reports %s, but the Effect-delivery reader consumes it", label)
		}
	}
}
