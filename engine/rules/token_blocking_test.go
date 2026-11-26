package rules

// TokenBlocking$ is Forge's "put the token onto the battlefield blocking that
// creature" rider (CR 509.4/509.4a): Brimaz, King of Oreskos's Cat Soldier
// enters as a blocker of the attacker Brimaz blocked. The token is not
// "declared" (CR 509.4b ignores block requirements/restrictions and fires no
// block trigger of its own), but it is a blocking creature, so the attacker
// is blocked by two creatures and its controller must divide its combat
// damage between them (CR 510.1c).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// tokenBlockingCat returns the first Cat Soldier token on any battlefield,
// or 0 when none exists yet.
func tokenBlockingCat(e *Engine) state.ObjID {
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone != state.ZBattlefield || !o.IsToken || o.Face() == nil {
			continue
		}
		types := strings.Join(o.Face().Types, " ")
		if strings.Contains(types, "Cat") && strings.Contains(types, "Soldier") {
			return o.ID
		}
	}
	return 0
}

// tokenBlockingDrive answers the engine until stop holds: priorities pass,
// trigger-order asks submit the offered order. Any other decision is
// unexpected in this fixture (there is no replacement or target ask in the
// Brimaz block shape).
func tokenBlockingDrive(t *testing.T, e *Engine, stop func() bool) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if stop() {
			return
		}
		if e.G.Over {
			t.Fatalf("game ended before the combat damage step")
		}
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
					break
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("pass priority: %v", err)
			}
		case decision.KTriggerOrder:
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("submit trigger order: %v", err)
			}
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
	}
	t.Fatal("combat did not reach the asserted state within the ask budget")
}

// TestTokenEntersBlockingTriggeredAttacker is the repro: p0 attacks with
// Grizzly Bears, p1 blocks with Brimaz, and the block trigger's Cat Soldier
// must enter blocking the Bears, so the Bears' controller is asked to divide
// two damage between Brimaz and the Cat.
func TestTokenEntersBlockingTriggeredAttacker(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg,
		[]string{"Grizzly Bears"}, nil,
		[]string{"Brimaz, King of Oreskos"}, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	bears := findBattlefield(t, e, 0, "Grizzly Bears", 0)
	brimaz := findBattlefield(t, e, 1, "Brimaz, King of Oreskos", 0)

	// Preconditions the rule reads: the attacker is on the battlefield and
	// attack-ready, the blocker is a creature Brimaz's own trigger matches.
	if o := e.G.Obj(bears); o == nil || o.Zone != state.ZBattlefield || o.IsAttacking || o.SummonSick {
		t.Fatalf("precondition: Bears = %+v, want battle-ready battlefield attacker", o)
	}
	if o := e.G.Obj(brimaz); o == nil || o.Zone != state.ZBattlefield || !strings.Contains(strings.Join(o.Face().Types, " "), "Creature") {
		t.Fatalf("precondition: Brimaz = %+v, want a battlefield creature", o)
	}

	e.askAttackers()
	submitAttackers(t, e, bears)
	if o := e.G.Obj(bears); !o.IsAttacking || o.Attacking != 1 {
		t.Fatalf("precondition: Bears not attacking seat 1: %+v", o)
	}
	submitBlockersOnly(t, e, brimaz)
	if got := e.G.Obj(bears).BlockedBy; len(got) != 1 || got[0] != brimaz {
		t.Fatalf("precondition: Bears blocked by %v, want [%d] (Brimaz)", got, brimaz)
	}
	if !e.G.Obj(brimaz).IsBlocking {
		t.Fatal("precondition: Brimaz is not blocking")
	}

	// Resolve Brimaz's block trigger, which mints the Cat Soldier.
	tokenBlockingDrive(t, e, func() bool { return tokenBlockingCat(e) != 0 })
	cat := tokenBlockingCat(e)
	if o := e.G.Obj(cat); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: Cat = %+v, want a battlefield token under seat 1", o)
	}

	// The rule: the token enters blocking the Bears.
	if !e.G.Obj(cat).IsBlocking {
		t.Fatalf("Cat Soldier IsBlocking=false, want true (CR 509.4a)")
	}
	if got := e.G.Obj(bears).BlockedBy; len(got) != 2 || got[0] != brimaz || got[1] != cat {
		t.Fatalf("Bears BlockedBy = %v, want [%d %d] (Brimaz then the Cat)", got, brimaz, cat)
	}

	// The consequence: two blockers means the Bears' controller divides its
	// two damage (CR 510.1c).
	var division *decision.Decision
	tokenBlockingDrive(t, e, func() bool {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			return false
		}
		for _, o := range d.Options {
			if o.Kind == "division" {
				division = d
				return true
			}
		}
		return false
	})
	if division == nil {
		t.Fatal("no combat damage division decision was posed over Brimaz and the Cat")
	}
	if division.Player != 0 {
		t.Fatalf("division asked of seat %d, want the Bears' controller seat 0", division.Player)
	}
	if len(e.combatRound.askOptions) == 0 {
		t.Fatal("division decision has no per-blocker splits")
	}
	for _, split := range e.combatRound.askOptions {
		if len(split) != 2 {
			t.Fatalf("division split %v covers %d blockers, want both", split, len(split))
		}
	}
	replayCheck(t, e, cfg)
}
