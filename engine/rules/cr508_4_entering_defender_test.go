package rules

// CR 508.4 (cr-g11b-entering-attacking-defender-choice): "If a creature is
// put onto the battlefield attacking, its controller chooses which defending
// player, planeswalker a defending player controls, or battle a defending
// player protects it's attacking ... (unless the effect ... specifies what
// it's attacking)." Hero of Bladehold (repo deck cavalry-charge.json)
// creates two Soldiers "tapped and attacking" without naming a defender, so
// with a planeswalker on the defending side its controller is asked.
// Forge corroborates: SpellAbilityEffect.addToCombat
// (SpellAbilityEffect.java:758-790) has the activating player choose from
// combat.getDefenders() for "True".

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// drainAnsweringEnteringDefender drains the stack like passUntilStackEmpty,
// answering every CR 508.4 "entering_defender" ask with pick(d) (an option
// Index) and counting the asks.
func drainAnsweringEnteringDefender(t *testing.T, e *Engine, limit int, pick func(*decision.Decision) int) int {
	t.Helper()
	asked := 0
	for i := 0; i < limit && !e.G.Over && len(e.G.Stack) > 0; i++ {
		if d := e.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "entering_defender" {
			asked++
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick(d)}}); err != nil {
				t.Fatalf("submit entering_defender: %v", err)
			}
			continue
		}
		passUntilStackEmpty(t, e, 1)
	}
	return asked
}

// drainFirstEnteringDefender is drainAnsweringEnteringDefender answering
// option 0 -- the trigger's own defending player, the R-9 stand-in -- for
// the multiplayer drains written before CR 508.4's ask existed.
func drainFirstEnteringDefender(t *testing.T, e *Engine, limit int) {
	t.Helper()
	drainAnsweringEnteringDefender(t, e, limit, func(d *decision.Decision) int { return d.Options[0].Index })
}

// heroAttacksWithWalker puts Hero of Bladehold on seat 0 and, when walker,
// Jace, the Mind Sculptor (with loyalty) on seat 1; Hero attacks seat 1 and
// the battle cry / token triggers go on the stack.
func heroAttacksWithWalker(t *testing.T, walker bool) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	var extras1 []*cards.Card
	if walker {
		extras1 = append(extras1, lookup(t, reg, "Jace, the Mind Sculptor"))
	}
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Hero of Bladehold")}, extras1)
	hero := moveByName(t, e, 0, "Hero of Bladehold", state.ZBattlefield)
	var jace state.ObjID
	if walker {
		jace = moveByName(t, e, 1, "Jace, the Mind Sculptor", state.ZBattlefield)
		e.emit(events.Event{Kind: events.CounterChange, Obj: jace, Counter: "LOYALTY", Amount: 3})
		if o := e.G.Obj(jace); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
			t.Fatalf("precondition: Jace = %+v, want battlefield under seat 1", o)
		}
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{hero}})
	e.putTriggersOnStack()
	answerTriggerOrders(t, e)
	if len(e.G.Stack) == 0 {
		t.Fatal("Hero of Bladehold's triggers did not reach the stack")
	}
	return e, cfg, jace
}

func heroSoldiers(e *Engine) []*state.Object {
	var out []*state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Soldier Token" {
			out = append(out, o)
		}
	}
	return out
}

// TestCR508_4HeroTokensAttackTheChosenPlaneswalker is the failing pin: with
// Jace on seat 1's battlefield, seat 0 is asked (player 1 first, the
// trigger's defender; then Jace) and both Soldiers attack the chosen Jace.
func TestCR508_4HeroTokensAttackTheChosenPlaneswalker(t *testing.T) {
	t.Parallel()
	e, cfg, jace := heroAttacksWithWalker(t, true)
	var offered []decision.Option
	asked := drainAnsweringEnteringDefender(t, e, 40, func(d *decision.Decision) int {
		offered = d.Options
		if d.Player != 0 {
			t.Fatalf("entering_defender asked of seat %d, want the tokens' controller 0", d.Player)
		}
		for _, o := range d.Options {
			if o.Obj == jace {
				return o.Index
			}
		}
		t.Fatalf("entering_defender options %+v do not offer Jace %d", d.Options, jace)
		return 0
	})
	if asked != 1 {
		t.Fatalf("entering_defender asked %d times, want once per Token resolution (CR 508.4)", asked)
	}
	if len(offered) != 2 || offered[0].Kind != "player" || offered[0].Player != 1 || offered[1].Obj != jace || offered[1].Player != 1 {
		t.Fatalf("options = %+v, want [player 1, Jace %d under seat 1]", offered, jace)
	}
	soldiers := heroSoldiers(e)
	if len(soldiers) != 2 {
		t.Fatalf("Hero minted %d Soldiers, want 2", len(soldiers))
	}
	for _, s := range soldiers {
		if !s.Tapped || !s.IsAttacking || s.Attacking != 1 || s.AttackingBattle != jace {
			t.Fatalf("Soldier tapped=%v attacking=%v defender=%d battle=%d, want tapped, attacking Jace %d (seat 1)",
				s.Tapped, s.IsAttacking, s.Attacking, s.AttackingBattle, jace)
		}
	}
	replayCheck(t, e, cfg)
}

// TestCR508_4HeroAsksNothingWithOneDefender is the guard: with only the
// defending player to attack, no decision is posed (a decision nobody could
// answer differently is never emitted) and the Soldiers attack seat 1.
func TestCR508_4HeroAsksNothingWithOneDefender(t *testing.T) {
	t.Parallel()
	e, cfg, _ := heroAttacksWithWalker(t, false)
	asked := drainAnsweringEnteringDefender(t, e, 40, func(d *decision.Decision) int {
		t.Fatalf("unexpected entering_defender ask %+v with one defender", d)
		return 0
	})
	if asked != 0 {
		t.Fatalf("asked %d times, want 0", asked)
	}
	soldiers := heroSoldiers(e)
	if len(soldiers) != 2 {
		t.Fatalf("Hero minted %d Soldiers, want 2", len(soldiers))
	}
	for _, s := range soldiers {
		if !s.IsAttacking || s.Attacking != 1 || s.AttackingBattle != 0 {
			t.Fatalf("Soldier attacking=%v defender=%d battle=%d, want attacking player 1", s.IsAttacking, s.Attacking, s.AttackingBattle)
		}
	}
	replayCheck(t, e, cfg)
}
