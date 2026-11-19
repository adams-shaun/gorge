package rules

// willie_lumpkin_test.go — Willie Lumpkin, Postman's Effect-delivered "that
// player can't attack you or permanents you control during their next turn":
// the Effect's `RememberObjects$ You` is the RESOLVING controller (Willie's), but
// the registered restriction belongs to `EffectOwner$ TriggeredTarget` (the
// damaged player), whose creatures `Creature.YouCtrl` selects.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const willieWalkerFixture = "Name:Willie Probe Walker\nTypes:Planeswalker\nLoyalty:3\nOracle:x\n"

// willieBoard: seat 0 Willie + a planeswalker, seats 1 and 2 one bear each.
// Willie deals combat damage to seat 1, and seat 1 answers the draw prompt.
func willieBoard(t *testing.T, accept bool) (e *Engine, cfg Config, bear1, bear2, walker, willie state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	w, ok := reg.Lookup("Willie Lumpkin, Postman")
	if !ok {
		t.Fatal("corpus missing Willie Lumpkin, Postman")
	}
	b1, b2 := card(t, staticBearFixture), card(t, staticBearFixture)
	pw := card(t, willieWalkerFixture)
	e, cfg = restrictionGame(t, 6120,
		[][]*cards.Card{nil, nil, nil},
		[][]*cards.Card{{w, pw}, {b1}, {b2}})
	willie = findOnBoard(t, e, 0, "Willie Lumpkin, Postman")
	walker = findOnBoard(t, e, 0, "Willie Probe Walker")
	bear1, bear2 = bearOnBoard(t, e, 1, b1), bearOnBoard(t, e, 2, b2)
	if e.attackBlocked(bear1, 0, 0) || e.attackBlocked(bear1, 0, walker) {
		t.Fatal("precondition: seat 1's bear is already restricted")
	}

	handBefore := len(e.G.Zone(state.ZHand, 1))
	prev := e.SetDamageSource(willie)
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 1})
	e.combatDamaging = false
	e.SetDamageSource(prev)
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after combat damage = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	if len(e.G.Stack) == 0 {
		t.Fatal("precondition: the DamageDone trigger did not reach the stack")
	}
	// Pass priority until seat 1 is asked whether to draw (the UnlessCost$
	// Draw<1/...> payment, UnlessSwitched$ True: paying is what avoids nothing,
	// it is what makes the restriction apply), then answer it.
	asked := false
	for i := 0; i < 60 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision with stack %v", e.G.Stack)
		}
		choice := 0
		if d.Kind == decision.KModes && d.ResumeKind == "unless_pay" {
			asked = true
			if d.Player != 1 {
				t.Fatalf("unless_pay asked of seat %d, want 1", d.Player)
			}
			want := decision.ModeUnlessDecline
			if accept {
				want = decision.ModeUnlessPay
			}
			choice = -1
			for _, o := range d.Options {
				if o.Mode == want {
					choice = o.Index
				}
			}
			if choice < 0 {
				t.Fatalf("no %s option: %+v", want, d.Options)
			}
		} else if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{choice}}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	if !asked {
		t.Fatal("precondition: seat 1 was never asked whether to draw")
	}
	wantHand := handBefore
	if accept {
		wantHand++
	}
	if got := len(e.G.Zone(state.ZHand, 1)); got != wantHand {
		t.Fatalf("seat 1 hand = %d, want %d (accept=%v): the draw cost was not paid/declined as answered", got, wantHand, accept)
	}
	return
}

func TestWillieLumpkinAcceptedDrawRestrictsOnlyAttacksOnController(t *testing.T) {
	t.Parallel()
	e, cfg, bear1, bear2, walker, willie := willieBoard(t, true)
	if got := e.G.Obj(willie).Controller; got != 0 {
		t.Fatalf("precondition: Willie controller = %d", got)
	}
	if !e.attackBlocked(bear1, 0, 0) {
		t.Fatal("seat 1's creature may attack seat 0 after drawing: the restriction did not bind")
	}
	if e.attackBlocked(bear1, 2, 0) {
		t.Fatal("seat 1's creature is barred from attacking seat 2 (the other player)")
	}
	if e.attackBlocked(bear2, 0, 0) {
		t.Fatal("seat 2's creature is barred: the restriction is scoped to the wrong seat")
	}
	if !e.attackBlocked(bear1, 0, walker) {
		t.Fatal("seat 1's creature may attack seat 0's planeswalker")
	}
	if e.attackBlocked(willie, 1, 0) {
		t.Fatal("Willie's own controller's creature is restricted")
	}
	// It lasts through seat 1's NEXT turn (seat 0's turn 2 passes first).
	drainThrough(t, e, 3, 1, state.StepDeclareAttackers)
	if !e.attackBlocked(bear1, 0, 0) {
		t.Fatal("restriction gone during seat 1's next turn")
	}
	drainThrough(t, e, 4, 2, state.StepUpkeep)
	if e.attackBlocked(bear1, 0, 0) {
		t.Fatal("restriction outlived seat 1's next turn")
	}
	replayCheck(t, e, cfg)
}

func TestWillieLumpkinDeclinedDrawRestrictsNothing(t *testing.T) {
	t.Parallel()
	e, _, bear1, _, walker, _ := willieBoard(t, false)
	if e.attackBlocked(bear1, 0, 0) || e.attackBlocked(bear1, 0, walker) || e.attackBlocked(bear1, 2, 0) {
		t.Fatal("seat 1 declined to draw yet is restricted")
	}
}
