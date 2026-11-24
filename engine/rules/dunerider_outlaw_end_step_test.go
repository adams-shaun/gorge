package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDuneriderOutlawEndStepCounterAfterOpponentDamage: Dunerider Outlaw's
// intervening-if end-step trigger (IsPresent$ Card.Self+dealtDamageToOppThisTurn,
// CR 603.4) goes on the stack only when the Outlaw dealt damage to an
// opponent this turn. Attacking seat 1 unblocked deals 1 combat damage, so
// the trigger fires and resolves the 1/1 to a 2/2 with one +1/+1 counter;
// declining the attack leaves the end-step stack empty.
func TestDuneriderOutlawEndStepCounterAfterOpponentDamage(t *testing.T) {
	t.Parallel()
	for _, attack := range []bool{true, false} {
		name := "no-attack"
		if attack {
			name = "attacks-opponent"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, cfg := b5Engine(t, "Dunerider Outlaw")
			outlaw := searchMoveByName(t, e, "Dunerider Outlaw", state.ZBattlefield)
			e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
			e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
			e.pending = nil
			e.priorityRound()
			driveToAttackers(t, e)
			d := e.Pending()
			var choices []int
			if attack {
				for _, o := range d.Options {
					if o.Obj == outlaw && o.Player == 1 {
						choices = append(choices, o.Index)
					}
				}
				if len(choices) != 1 {
					t.Fatalf("Dunerider Outlaw cannot attack seat 1: %+v", d.Options)
				}
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("submit attackers: %v", err)
			}
			life := e.G.Players[1].Life
			duneriderDriveToEndStepPriority(t, e)
			wantLife := life
			if attack {
				wantLife = life - 1
			}
			// Precondition the trigger rests on: the combat damage landed
			// (or, declining, did not).
			if got := e.G.Players[1].Life; got != wantLife {
				t.Fatalf("seat 1 life = %d at the end step, want %d", got, wantLife)
			}
			var triggers int
			for _, id := range e.G.Stack {
				if o := e.G.Obj(id); o != nil && o.Ability != nil && o.Source == outlaw {
					triggers++
				}
			}
			if !attack {
				if triggers != 0 || len(e.G.Stack) != 0 {
					t.Fatalf("no opponent damage, but the end-step stack = %v", e.G.Stack)
				}
				return
			}
			if triggers != 1 {
				t.Fatalf("Dunerider Outlaw's end-step trigger: %d on the stack %v, want 1", triggers, e.G.Stack)
			}
			driveStackEmpty(t, e, 20, func(*decision.Decision) int { return 0 })
			if n := e.G.Obj(outlaw).Counter("P1P1"); n != 1 {
				t.Fatalf("P1P1 counters = %d, want 1", n)
			}
			if p, tg := e.Power(outlaw), e.Toughness(outlaw); p != 2 || tg != 2 {
				t.Fatalf("Dunerider Outlaw is %d/%d, want 2/2", p, tg)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// duneriderDriveToEndStepPriority passes priority (and declines any blocks)
// until seat 0's turn reaches its end step with a priority decision pending,
// after the beginning-of-step triggers went on the stack.
func duneriderDriveToEndStepPriority(t *testing.T, e *Engine) {
	t.Helper()
	turn := e.G.Turn
	for i := 0; i < 200 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			d = e.Pending()
		}
		if d == nil {
			t.Fatal("no decision pending")
		}
		if e.G.Turn == turn && e.G.Step == state.StepEnd && d.Kind == decision.KPriority {
			return
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e)
		case decision.KBlockers:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player}); err != nil {
				t.Fatalf("submit blockers: %v", err)
			}
		default:
			t.Fatalf("unexpected decision %+v while driving to the end step", d)
		}
	}
	t.Fatalf("did not reach the end step: turn=%d step=%s", e.G.Turn, e.G.Step)
}
