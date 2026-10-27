package rules

// Pump's ThisDefinedAndTgts$ (Forge SpellAbilityEffect.getCards): the pump
// affects the named extra objects AND the chosen targets, once each, sharing
// one KWChoice$ answer.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	ballroomBrawlersSrc = "Name:Ballroom Brawlers\nManaCost:2 W\nTypes:Creature Human Warrior\nPT:3/3\nK:Haste\n" +
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigPump | TriggerDescription$ x\n" +
		"SVar:TrigPump:DB$ Pump | ValidTgts$ Creature.Other+YouCtrl | TargetMin$ 0 | TargetMax$ 1 | ThisDefinedAndTgts$ Self | KWChoice$ First Strike,Lifelink\n" +
		"Oracle:x\n"
	razorgrassInvokerSrc = "Name:Razorgrass Invoker\nManaCost:1 G\nTypes:Creature Elemental\nPT:2/2\n" +
		"A:AB$ Pump | Cost$ 1 | Defined$ Self | ValidTgts$ Creature.Other | TargetMin$ 0 | TargetMax$ 1 | ThisDefinedAndTgts$ Self | NumAtt$ +3 | NumDef$ +3\n" +
		"Oracle:x\n"
	pumpPlainBearSrc = "Name:Pump Plain Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	pumpGiantGrowth  = "Name:Giant Growth\nManaCost:G\nTypes:Instant\n" +
		"A:SP$ Pump | ValidTgts$ Creature | NumAtt$ +3 | NumDef$ +3\nOracle:x\n"
)

// answerPumpAsks drains the pending asks until the stack is empty: a KTarget
// ask picks the option naming target (or declines with no choice when target
// is 0), a KModes ask picks the option labelled kw.
func answerPumpAsks(t *testing.T, e *Engine, target state.ObjID, kw string) {
	t.Helper()
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			continue
		}
		switch d.Kind {
		case decision.KTarget:
			var choices []int
			for _, o := range d.Options {
				if target != 0 && o.Obj == target {
					choices = append(choices, o.Index)
				}
			}
			if target != 0 && len(choices) == 0 {
				t.Fatalf("target %d not offered: %+v", target, d.Options)
			}
			submitChoices(t, e, choices...)
		case decision.KModes:
			pick := -1
			for _, o := range d.Options {
				if o.Label == kw {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("keyword %q not offered: %+v", kw, d.Options)
			}
			submitChoices(t, e, pick)
		case decision.KPriority:
			if len(e.G.Stack) == 0 {
				return
			}
			passOnce(t, e)
		default:
			t.Fatalf("unexpected pending %+v", d)
		}
	}
	t.Fatalf("asks never drained; stack %v", e.G.Stack)
}

func ballroomAttack(t *testing.T, seed uint64, withTarget bool) (e *Engine, cfg Config, brawlers, other state.ObjID) {
	t.Helper()
	e, cfg, brawlers = newFixtureDeck(t, seed, ballroomBrawlersSrc, pumpPlainBearSrc)
	toMain1(t, e)
	e.emit(events.Event{Kind: events.MoveZone, Obj: brawlers, From: e.G.Obj(brawlers).Zone, To: state.ZBattlefield})
	other = putCreature(t, e, 0, pumpPlainBearSrc)
	e.pending = nil
	e.priorityRound()
	if e.G.Obj(brawlers).Zone != state.ZBattlefield || e.G.Obj(other).Zone != state.ZBattlefield {
		t.Fatal("precondition: both creatures on the battlefield")
	}
	for _, id := range []state.ObjID{brawlers, other} {
		if e.HasKeyword(id, "Lifelink") || e.HasKeyword(id, "First Strike") {
			t.Fatalf("precondition: %d already has a keyword", id)
		}
	}
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, brawlers)
	tgt := other
	if !withTarget {
		tgt = 0
	}
	answerPumpAsks(t, e, tgt, "Lifelink")
	return e, cfg, brawlers, other
}

func TestPumpThisDefinedAndTgtsBallroomBothGainChoice(t *testing.T) {
	t.Parallel()
	e, cfg, brawlers, other := ballroomAttack(t, 6301, true)
	for _, id := range []state.ObjID{brawlers, other} {
		if !e.HasKeyword(id, "Lifelink") {
			t.Fatalf("%d lacks the chosen Lifelink: %v", id, e.Derived(id).Keywords)
		}
		if e.HasKeyword(id, "First Strike") {
			t.Fatalf("%d gained the unchosen First Strike", id)
		}
	}
	e.EndOfTurnCleanup()
	if e.HasKeyword(brawlers, "Lifelink") || e.HasKeyword(other, "Lifelink") {
		t.Fatal("the grant outlived the turn")
	}
	replayCheck(t, e, cfg)
}

func TestPumpThisDefinedAndTgtsBallroomNoTarget(t *testing.T) {
	t.Parallel()
	e, cfg, brawlers, other := ballroomAttack(t, 6302, false)
	if !e.HasKeyword(brawlers, "Lifelink") {
		t.Fatalf("Ballroom Brawlers lacks Lifelink with no target: %v", e.Derived(brawlers).Keywords)
	}
	if e.HasKeyword(other, "Lifelink") {
		t.Fatal("the untargeted creature gained Lifelink")
	}
	replayCheck(t, e, cfg)
}

func TestPumpThisDefinedAndTgtsRazorgrassPumpsBoth(t *testing.T) {
	t.Parallel()
	e, cfg, inv := newFixtureDeck(t, 6303, razorgrassInvokerSrc, pumpPlainBearSrc)
	toMain1(t, e)
	e.emit(events.Event{Kind: events.MoveZone, Obj: inv, From: e.G.Obj(inv).Zone, To: state.ZBattlefield})
	other := putCreature(t, e, 0, pumpPlainBearSrc)
	addMana(t, e, 0, "R")
	e.pending = nil
	e.priorityRound()
	if e.Power(inv) != 2 || e.Power(other) != 2 {
		t.Fatalf("precondition: powers %d/%d, want 2/2", e.Power(inv), e.Power(other))
	}
	abilityIdx := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == inv {
			abilityIdx = o.Index
		}
	}
	if abilityIdx < 0 {
		t.Fatalf("Razorgrass Invoker's ability not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, abilityIdx)
	answerPumpAsks(t, e, other, "")
	if e.Power(other) != 5 || e.Toughness(other) != 5 {
		t.Fatalf("target is %d/%d, want 5/5", e.Power(other), e.Toughness(other))
	}
	if e.Power(inv) != 5 || e.Toughness(inv) != 5 {
		t.Fatalf("Razorgrass Invoker is %d/%d, want 5/5 (pumped once, not twice)", e.Power(inv), e.Toughness(inv))
	}
	replayCheck(t, e, cfg)
}

func TestPumpWithoutThisDefinedAndTgtsUnchanged(t *testing.T) {
	t.Parallel()
	e, cfg, gg := newFixtureDeck(t, 6304, pumpGiantGrowth, pumpPlainBearSrc, pumpPlainBearSrc)
	toMain1(t, e)
	a := putCreature(t, e, 0, pumpPlainBearSrc)
	b := putCreature(t, e, 0, pumpPlainBearSrc)
	addMana(t, e, 0, "G")
	e.pending = nil
	e.priorityRound()
	if e.G.Obj(gg).Zone != state.ZHand {
		t.Fatalf("precondition: Giant Growth in %v, not hand", e.G.Obj(gg).Zone)
	}
	castIdx := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == gg {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("no cast option for Giant Growth: %+v", e.Pending().Options)
	}
	submitChoices(t, e, castIdx)
	answerPumpAsks(t, e, a, "")
	hit := 0
	for _, id := range []state.ObjID{a, b} {
		if e.Power(id) == 5 {
			hit++
		}
	}
	if hit != 1 {
		t.Fatalf("%d of the two bears were pumped, want exactly one (powers %d/%d)", hit, e.Power(a), e.Power(b))
	}
	replayCheck(t, e, cfg)
}
