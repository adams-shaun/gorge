package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func playerSetBoard(t *testing.T, names []string, spec string, broad bool) (*Engine, state.ObjID) {
	t.Helper()
	decks := make([][]*cards.Card, len(names))
	for i := range decks {
		decks[i] = mountainDeck(t, 40)
	}
	e := New(seatZeroStart(Config{Seed: 1, Names: names, Decks: decks}))
	e.G.Active = 1
	if len(names) == 2 {
		e.G.Active = 1
	}
	e.G.Step = state.StepDeclareAttackers
	c := onBoardReady(t, e, 1, "Name:Dragon\nTypes:Creature\nPT:5/5\nOracle:x\n")
	src := onBoardReady(t, e, 0, "Name:Saga\nTypes:Creature\nPT:1/1\nOracle:x\n")
	if o := e.G.Obj(c); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: creature not on the battlefield")
	}
	reg := func(params map[string]string) {
		e.AddContinuous(ContinuousEffect{Source: src, Controller: 0, UntilEOT: true, Restriction: "MustAttack", RestrictParams: params})
	}
	reg(map[string]string{"Mode": "MustAttack", "ValidCreature": "Creature", "MustAttack": spec})
	if broad {
		reg(map[string]string{"Mode": "MustAttack", "ValidCreature": "Creature"})
	}
	return e, c
}

func TestMustAttackPlayerOtherNamesEveryOtherPlayer(t *testing.T) {
	t.Parallel()
	e, c := playerSetBoard(t, []string{"a", "b", "c"}, "Player.Other", true)
	rs := e.attackRequirements(c)
	if rs.SatisfiedBy(2) != 1 || rs.SatisfiedBy(0) != 0 || rs.SatisfiedBy(1) != 0 {
		t.Fatalf("Player.Other should name only seat 2 (not controller 0, not active 1): %+v", rs.Named)
	}
	if !e.requiredForDefender(c, 2) || e.requiredForDefender(c, 0) {
		t.Fatalf("seat 2 must be required and the controller not")
	}
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected KAttackers, got %+v", d)
	}
	seen := 0
	for _, opt := range d.Options {
		if opt.Obj != c {
			continue
		}
		seen++
		if want := opt.Player == 2; opt.Required != want {
			t.Fatalf("pair against %d Required=%v, want %v", opt.Player, opt.Required, want)
		}
	}
	if seen == 0 {
		t.Fatalf("precondition: no options for the creature: %+v", d.Options)
	}
}

func TestMustAttackPlayerOtherTwoPlayersStillAttacks(t *testing.T) {
	t.Parallel()
	e, c := playerSetBoard(t, []string{"a", "b"}, "Player.Other", true)
	rs := e.attackRequirements(c)
	if !rs.Broad {
		t.Fatalf("precondition: the Broad duty must bind: %+v", rs)
	}
	if rs.SatisfiedBy(0) != 0 || len(rs.Named) != 0 {
		t.Fatalf("no named count expected with only the controller to attack: %+v", rs.Named)
	}
	if !e.mustAttackRequired(c) {
		t.Fatal("the creature must still be required to attack")
	}
}

func TestMustAttackDefinedReferenceStaysFailClosed(t *testing.T) {
	t.Parallel()
	e, c := playerSetBoard(t, []string{"a", "b", "c"}, "CardOwner", false)
	rs := e.attackRequirements(c)
	if len(rs.Named) != 0 || rs.Any() {
		t.Fatalf("a defined reference must add nothing: %+v", rs)
	}
}
