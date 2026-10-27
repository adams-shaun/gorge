package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
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
