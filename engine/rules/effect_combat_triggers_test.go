package rules

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	ectBear   = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	ectWhite2 = "Name:White Knight\nManaCost:W W\nColors:white\nTypes:Creature Knight\nPT:2/2\nOracle:x\n"
)

// ectCast resolves a corpus card's Effect body as seat `you` (the card sits on
// the battlefield only as the registration's source object; its zone is
// irrelevant to an Effect, CR 611.2a). svar names a sub-ability body, or ""
// for the card's own SP$.
func ectCast(t *testing.T, e *Engine, c *cards.Card, you state.PlayerID, svar string) state.ObjID {
	t.Helper()
	src := onBoardCard(t, e, you, c)
	sa := c.Faces[0].SpellAbility()
	if svar != "" {
		sa = cards.ResolveSVar(c.Faces[0].SVars, svar)
	}
	if sa == nil {
		t.Fatalf("%s: ability %q unresolved", c.Faces[0].Name, svar)
	}
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: you, SVars: c.Faces[0].SVars}, sa)
	return src
}

func ectEffectRegs(e *Engine, mode string) int {
	n := 0
	for _, dt := range e.G.Delayed {
		if dt.EffectRepeat && dt.EventMode == mode {
			n++
		}
	}
	return n
}

func ectNoUnimplemented(t *testing.T, e *Engine) {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented") {
			t.Fatalf("unexpected Note: %s", ev.Text)
		}
	}
}

// ectCloneAgrees resolves the top of the stack on the engine and a clone and
// requires identical event streams (the eventless fixture board cannot be
// rebuilt from the log alone, so replayCheck does not apply).
func ectCloneAgrees(t *testing.T, e *Engine) {
	t.Helper()
	clone := e.Clone()
	e.resolveTop()
	clone.resolveTop()
	if !reflect.DeepEqual(e.L.Events, clone.L.Events) {
		t.Fatal("clone diverged across the Effect trigger's resolution")
	}
}

func TestEffectBlocksTriggerBattleCry(t *testing.T) {
	t.Parallel()
	e := blocksCombatEngine(t, 1)
	ectCast(t, e, unblockedCorpusCard(t, "b/battle_cry.txt"), 0, "Battlecry")
	ectNoUnimplemented(t, e)
	if n := ectEffectRegs(e, "Blocks"); n != 1 {
		t.Fatalf("Blocks |EF registrations = %d, want 1", n)
	}
	atk := onBoardReady(t, e, 1, ectBear)
	blk := onBoard(t, e, 0, ectBear)
	if e.Toughness(blk) != 2 {
		t.Fatalf("blocker precondition toughness = %d, want 2", e.Toughness(blk))
	}
	e.askAttackers()
	submitAttackersOnly(t, e, atk)
	drainCombatPriority(t, e)
	submitBlockersOnly(t, e, blk)
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack after blocks = %d, want 1 trigger", len(e.G.Stack))
	}
	ectCloneAgrees(t, e)
	if e.Toughness(blk) != 3 || e.Power(blk) != 2 {
		t.Fatalf("blocker after Battle Cry = %d/%d, want 2/3", e.Power(blk), e.Toughness(blk))
	}
	ectNoUnimplemented(t, e)
}

// TestEffectBlockTriggerExpires: the grant lasts the turn; a block on a later
// turn fires nothing.
func TestEffectBlockTriggerExpires(t *testing.T) {
	t.Parallel()
	e := blocksCombatEngine(t, 1)
	ectCast(t, e, unblockedCorpusCard(t, "b/battle_cry.txt"), 0, "Battlecry")
	if n := ectEffectRegs(e, "Blocks"); n != 1 {
		t.Fatalf("Blocks |EF registrations = %d, want 1", n)
	}
	e.G.Turn++ // a later turn: the registration's MaxTurn is behind us
	atk := onBoardReady(t, e, 1, ectBear)
	blk := onBoard(t, e, 0, ectBear)
	e.askAttackers()
	submitAttackersOnly(t, e, atk)
	drainCombatPriority(t, e)
	submitBlockersOnly(t, e, blk)
	if len(e.G.Stack) != 0 || e.Toughness(blk) != 2 {
		t.Fatalf("expired grant fired: stack=%d toughness=%d", len(e.G.Stack), e.Toughness(blk))
	}
}

// TestEffectAttackerUnblockedGazeOfPain: two attackers, one blocked; exactly
// one optional trigger, for the unblocked one.
func TestEffectAttackerUnblockedGazeOfPain(t *testing.T) {
	t.Parallel()
	e := blocksCombatEngine(t, 0)
	ectCast(t, e, unblockedCorpusCard(t, "g/gaze_of_pain.txt"), 0, "")
	ectNoUnimplemented(t, e)
	if n := ectEffectRegs(e, "AttackerUnblocked"); n != 1 {
		t.Fatalf("AttackerUnblocked |EF registrations = %d, want 1", n)
	}
	a1 := onBoardReady(t, e, 0, ectBear)
	a2 := onBoardReady(t, e, 0, "Name:Runeclaw Bear Two\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	blk := onBoard(t, e, 1, "Name:Memnite\nManaCost:0\nTypes:Artifact Creature Construct\nPT:1/1\nOracle:x\n")
	e.askAttackers()
	submitAttackersOnly(t, e, a1, a2)
	drainCombatPriority(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	pair := [2]state.ObjID{a1, blk}
	submitBlockerPairsOnly(t, e, pair)
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want exactly one trigger (the unblocked attacker's)", e.G.Stack)
	}
	if tc := e.triggerContexts[e.G.Stack[0]]; tc.TriggerCard != a2 {
		t.Fatalf("trigger captured attacker %d, want the unblocked %d (blocked is %d)", tc.TriggerCard, a2, a1)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the trigger's target ask, got %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{2}}); err != nil {
		t.Fatalf("submit target: %v", err)
	}
	passAll := func() {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("expected a priority decision, got %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("pass: %v", err)
		}
	}
	passAll()
	passAll()
	d = e.Pending()
	if d == nil || len(d.Options) != 2 || d.Options[0].Kind != "yes" {
		t.Fatalf("expected the OptionalDecider yes/no ask, got %+v", d)
	}
}

// TestEffectAttackerBlockedRememberedOnly: Barreling Attack's remembered
// creature gets its pump when blocked; another blocked attacker does not.
func TestEffectAttackerBlockedRememberedOnly(t *testing.T) {
	t.Parallel()
	e := blocksCombatEngine(t, 0)
	c := unblockedCorpusCard(t, "b/barreling_attack.txt")
	a1 := onBoardReady(t, e, 0, ectBear)
	a2 := onBoardReady(t, e, 0, "Name:Runeclaw Bear Two\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b1 := onBoard(t, e, 1, "Name:Memnite\nManaCost:0\nTypes:Artifact Creature Construct\nPT:1/1\nOracle:x\n")
	b2 := onBoard(t, e, 1, "Name:Memnite Two\nManaCost:0\nTypes:Artifact Creature Construct\nPT:1/1\nOracle:x\n")
	src := onBoardCard(t, e, 0, c)
	sa := cards.ResolveSVar(c.Faces[0].SVars, "DBEffect")
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0, SVars: c.Faces[0].SVars,
		Remembered: []state.Target{{Obj: a1}}, Targets: []state.Target{{Obj: a1}}}, sa)
	ectNoUnimplemented(t, e)
	if n := ectEffectRegs(e, "AttackerBlocked"); n != 1 {
		t.Fatalf("AttackerBlocked |EF registrations = %d, want 1", n)
	}
	e.askAttackers()
	submitAttackersOnly(t, e, a1, a2)
	drainCombatPriority(t, e)
	submitBlockerPairsOnly(t, e, [2]state.ObjID{a1, b1}, [2]state.ObjID{a2, b2})
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %d, want exactly one trigger (the remembered attacker's)", len(e.G.Stack))
	}
	ectCloneAgrees(t, e)
	if e.Power(a1) != 3 || e.Power(a2) != 2 {
		t.Fatalf("powers a1=%d a2=%d, want 3 and 2", e.Power(a1), e.Power(a2))
	}
}
