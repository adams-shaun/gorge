package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// api:EndCombatPhase (CR 724.2): an effect that ends the combat phase exiles
// the whole stack (itself included), removes every creature from combat,
// expires "until end of combat" effects and skips to the postcombat main
// phase, so the end of combat step's own triggers never fire (724.2e).

const (
	ceasefireText = "Name:Test Ceasefire\nManaCost:0\nTypes:Instant\n" +
		"A:SP$ EndCombatPhase | SpellDescription$ The combat phase ends.\nOracle:x\n"
	optionalCeasefireText = "Name:Test Ceasefire Maybe\nManaCost:0\nTypes:Instant\n" +
		"A:SP$ EndCombatPhase | Optional$ True | SpellDescription$ You may end the combat phase.\nOracle:x\n"
	bruiserText = "Name:Test Bruiser\nManaCost:2 R\nTypes:Creature Ogre\nPT:2/2\n" +
		"K:Haste\n" +
		"A:AB$ Pump | Cost$ 0 | Defined$ Self | NumAtt$ +3 | Duration$ UntilEndOfCombat | SpellDescription$ Gets +3/+0 until end of combat.\n" +
		"T:Mode$ Phase | Phase$ EndCombat | TriggerZones$ Battlefield | Execute$ TrigMend | TriggerDescription$ At end of combat, you gain 3 life.\n" +
		"SVar:TrigMend:DB$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n"
	wallText = "Name:Test Wall\nManaCost:1 W\nTypes:Creature Wall\nPT:0/4\nOracle:x\n"
)

type endCombatFixture struct {
	e                         *Engine
	cfg                       Config
	atk, blk, chaplain, spell state.ObjID
	startLife                 int32
}

// newEndCombatFixture runs a combat to the declare blockers step's priority
// round: seat 0's pumped Bruiser is blocked by seat 1's Wall, seat 0 holds
// the spell and a Chaplain with an end-of-combat trigger sits on its board.
func newEndCombatFixture(t *testing.T, spellText string) endCombatFixture {
	t.Helper()
	e, cfg, _ := newFixtureDeckWithOpponentCard(t, 7, bruiserText, spellText, wallText)
	f := endCombatFixture{e: e, cfg: cfg}
	f.spell = addToHand(t, e, 0, spellText)
	f.atk = putCreature(t, e, 0, bruiserText)
	f.blk = moveSeeded(t, e, 1, wallText, state.ZBattlefield)
	f.startLife = e.G.Players[0].Life
	e.Advance()
	for i := 0; i < 60 && (e.Pending() == nil || e.Pending().Kind != decision.KAttackers); i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision on the way to combat: %+v", d)
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}
	submitAttackersOnly(t, e, f.atk)
	drainCombatPriority(t, e)
	if d := e.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected the blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e, f.blk)
	if e.G.Step != state.StepDeclareBlockers || !e.G.Obj(f.atk).IsAttacking {
		t.Fatalf("precondition: step %s, attacking %v", e.G.Step, e.G.Obj(f.atk).IsAttacking)
	}
	// The until-end-of-combat pump is live before the spell.
	face := e.G.Obj(f.atk).Face()
	e.resolveAbility(f.atk, 0, nil, face.Abilities[0], face.SVars)
	if got := e.Power(f.atk); got != 5 {
		t.Fatalf("precondition: pumped power = %d, want 5", got)
	}
	return f
}

func (f endCombatFixture) cast(t *testing.T) {
	t.Helper()
	d := f.e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected priority, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == f.spell {
			submitChoices(t, f.e, o.Index)
			return
		}
	}
	t.Fatalf("no cast option for the spell: %+v", d.Options)
}

func endCombatEvents(e *Engine) int {
	return countEvents(e, func(ev events.Event) bool { return ev.Kind == events.EndCombatPhase })
}

// TestEndCombatPhaseSkipsToPostcombatMain: the spell resolving mid-combat
// leaves the game in the second main phase with combat gone, the pump
// expired, the spell exiled and the end-of-combat trigger never fired.
func TestEndCombatPhaseSkipsToPostcombatMain(t *testing.T) {
	t.Parallel()
	f := newEndCombatFixture(t, ceasefireText)
	e := f.e
	f.cast(t)
	passUntilStackEmpty(t, e, 20)

	if n := endCombatEvents(e); n != 1 {
		t.Fatalf("EndCombatPhase events = %d, want 1", n)
	}
	if e.G.Step != state.StepMain2 {
		t.Fatalf("step = %s, want postcombat main (CR 724.2d)", e.G.Step)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want empty", e.G.Stack)
	}
	if o := e.G.Obj(f.spell); o.Zone != state.ZExile {
		t.Fatalf("the resolving spell is in zone %v, want exile (CR 724.2b)", o.Zone)
	}
	for _, id := range []state.ObjID{f.atk, f.blk} {
		if o := e.G.Obj(id); o.IsAttacking || len(o.BlockedBy) > 0 || o.Zone != state.ZBattlefield {
			t.Fatalf("creature %d still in combat or gone: %+v", id, o)
		}
	}
	if got := e.Power(f.atk); got != 2 {
		t.Fatalf("power after the phase ended = %d, want printed 2 (until-end-of-combat expired)", got)
	}
	if got := e.G.Players[0].Life; got != f.startLife {
		t.Fatalf("life = %d, want %d: the end-of-combat trigger must not fire (CR 724.2e)", got, f.startLife)
	}
	if e.G.Obj(f.atk).Damage != 0 || e.G.Obj(f.blk).Damage != 0 {
		t.Fatal("combat damage was dealt after the phase ended")
	}
	replayCheck(t, e, f.cfg)
}

// TestEndCombatPhaseControlRunsTheEndOfCombatTrigger is the contrast: without
// the spell the same board fires the end-of-combat trigger, so the absence
// above is the rule at work, not a dead trigger.
func TestEndCombatPhaseControlRunsTheEndOfCombatTrigger(t *testing.T) {
	t.Parallel()
	f := newEndCombatFixture(t, ceasefireText)
	e := f.e
	for i := 0; i < 80 && e.G.Step != state.StepMain2; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}
	if e.G.Step != state.StepMain2 {
		t.Fatalf("control did not reach postcombat main: %s", e.G.Step)
	}
	if got := e.G.Players[0].Life; got != f.startLife+3 {
		t.Fatalf("control life = %d, want %d (the trigger fires through a normal combat)", got, f.startLife+3)
	}
}

// TestEndCombatPhaseOptionalDeclineAndAccept: Merciful Mandate's "you may"
// shape. Declining leaves combat running (and the spell in the graveyard);
// accepting ends it.
func TestEndCombatPhaseOptionalDeclineAndAccept(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		answer int
		ends   bool
	}{{"decline", 1, false}, {"accept", 0, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newEndCombatFixture(t, optionalCeasefireText)
			e := f.e
			f.cast(t)
			for i := 0; i < 20; i++ {
				d := e.Pending()
				if d == nil {
					break
				}
				if d.ResumeKind == "endcombatphase_optional" {
					if d.Player != 0 || len(d.Options) != 2 {
						t.Fatalf("election = %+v", d)
					}
					submitChoices(t, e, tc.answer)
					break
				}
				for _, o := range d.Options {
					if o.Kind == "pass" {
						submitChoices(t, e, o.Index)
						break
					}
				}
			}
			n := endCombatEvents(e)
			if tc.ends {
				if n != 1 || e.G.Step != state.StepMain2 || e.G.Obj(f.atk).IsAttacking {
					t.Fatalf("accept: events %d, step %s, attacking %v", n, e.G.Step, e.G.Obj(f.atk).IsAttacking)
				}
			} else if n != 0 || e.G.Step == state.StepMain2 || !e.G.Obj(f.atk).IsAttacking {
				t.Fatalf("decline: events %d, step %s, attacking %v", n, e.G.Step, e.G.Obj(f.atk).IsAttacking)
			}
			replayCheck(t, e, f.cfg)
		})
	}
}

// TestEndCombatPhaseOutsideCombatDoesNothing: CR 724.2g.
func TestEndCombatPhaseOutsideCombatDoesNothing(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 7, ceasefireText)
	toMain1(t, e)
	id := moveSeeded(t, e, 0, ceasefireText, state.ZStack)
	e.resolveAbility(id, 0, nil, e.G.Obj(id).Face().Abilities[0], e.G.Obj(id).Face().SVars)
	if n := endCombatEvents(e); n != 0 || e.G.Step != state.StepMain1 {
		t.Fatalf("events %d, step %s: ending the combat phase outside combat must do nothing", n, e.G.Step)
	}
}
