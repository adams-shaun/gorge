package state

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

func testCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("t.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

func fourPlayer(t *testing.T) *Game {
	t.Helper()
	g := NewGame([]string{"alice", "bob", "carol", "dave"})
	bear := testCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	for p := PlayerID(0); p < 4; p++ {
		var lib []ObjID
		for i := 0; i < 10; i++ {
			lib = append(lib, g.AddObject(bear, p).ID)
		}
		g.SetZone(ZLibrary, p, lib)
	}
	return g
}

func TestObjectIDsAreDenseAndStable(t *testing.T) {
	g := fourPlayer(t)
	if len(g.Objs) != 40 {
		t.Fatalf("objects = %d, want 40", len(g.Objs))
	}
	for i := range g.Objs {
		id := ObjID(i + 1)
		if g.Obj(id).ID != id {
			t.Fatalf("Obj(%d).ID = %d", id, g.Obj(id).ID)
		}
	}
	if g.Obj(0) != nil {
		t.Error("ObjID 0 must be the nil object")
	}
	if g.Obj(ObjID(len(g.Objs)+1)) != nil {
		t.Error("out-of-range ObjID must be nil, not a panic")
	}
}

func TestZonesArePerPlayerAndOrdered(t *testing.T) {
	g := fourPlayer(t)
	for p := PlayerID(0); p < 4; p++ {
		lib := g.Zone(ZLibrary, p)
		if len(lib) != 10 {
			t.Fatalf("player %d library = %d", p, len(lib))
		}
		for i := 1; i < len(lib); i++ {
			if lib[i] <= lib[i-1] {
				t.Fatalf("player %d library lost insertion order", p)
			}
		}
		if len(g.Zone(ZHand, p)) != 0 {
			t.Fatalf("player %d hand should start empty", p)
		}
	}
}

func TestCloneIsDeepAndIndependent(t *testing.T) {
	g := fourPlayer(t)
	g.Players[1].Life = 12
	g.Obj(3).Tapped = true
	g.Obj(3).Counters = []Counter{{"P1P1", 2}}
	g.SetZone(ZHand, 0, []ObjID{1, 2})
	g.Stack = []ObjID{5}

	c := g.Clone()
	if c.Players[1].Life != 12 || !c.Obj(3).Tapped || c.Obj(3).Counter("P1P1") != 2 {
		t.Fatal("clone did not copy state")
	}
	c.Players[1].Life = 99
	c.Obj(3).Tapped = false
	c.Obj(3).Counters[0].N = 7
	c.SetZone(ZHand, 0, []ObjID{1})
	c.Stack = append(c.Stack, 6)

	if g.Players[1].Life != 12 {
		t.Error("clone shares Players backing array")
	}
	if !g.Obj(3).Tapped {
		t.Error("clone shares Objs backing array")
	}
	if g.Obj(3).Counter("P1P1") != 2 {
		t.Error("clone shares Counters backing array")
	}
	if len(g.Zone(ZHand, 0)) != 2 {
		t.Error("clone shares zone backing array")
	}
	if len(g.Stack) != 1 {
		t.Error("clone shares Stack backing array")
	}
}

func TestAliveHelpersSkipEliminatedSeats(t *testing.T) {
	g := fourPlayer(t)
	g.Players[1].Lost = true
	if got := g.AliveFrom(0); len(got) != 3 || got[0] != 0 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("AliveFrom(0) = %v", got)
	}
	if got := g.AliveFrom(2); len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 0 {
		t.Fatalf("AliveFrom(2) = %v", got)
	}
	if g.NextAlive(0) != 2 {
		t.Errorf("NextAlive(0) = %d, want 2", g.NextAlive(0))
	}
	if g.AliveCount() != 3 {
		t.Errorf("AliveCount = %d", g.AliveCount())
	}
	for i := range g.Players {
		g.Players[i].Lost = true
	}
	if g.AliveCount() != 0 || g.NextAlive(0) != 0 {
		t.Error("all-eliminated must not loop forever")
	}
}

func cardsFixture() (*cards.Card, error) {
	c, _ := cards.ParseBytes("t.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	c.Link()
	return c, nil
}

// TestStepValidRejectsOutOfRange is Task 23's totality fix: events/apply.go's
// StepChange case stores e.Step unvalidated, so a tampered log can reach
// Project (and rules/trigger.go's own Step.String() call) with an
// out-of-range step. Step.Valid mirrors Zone.Valid so callers fed an
// untrusted Step have the same guard available.
func TestStepValidRejectsOutOfRange(t *testing.T) {
	if !StepCleanup.Valid() {
		t.Error("the last defined step must be valid")
	}
	if !StepUntap.Valid() {
		t.Error("the first defined step must be valid")
	}
	if Step(numSteps).Valid() {
		t.Error("one past the last defined step must not be valid")
	}
	if Step(255).Valid() {
		t.Error("a wildly out-of-range step must not be valid")
	}
}

// TestStepStringIsTotal is the other half: stepNames[s] must never be reached
// with an out-of-range index, the same shape as events.Kind.String().
func TestStepStringIsTotal(t *testing.T) {
	if got := StepUpkeep.String(); got != "upkeep" {
		t.Errorf("StepUpkeep.String() = %q, want upkeep", got)
	}
	if got := Step(numSteps).String(); got != "unknown" {
		t.Errorf("an out-of-range step's String() = %q, want unknown", got)
	}
	if got := Step(255).String(); got != "unknown" {
		t.Errorf("a wildly out-of-range step's String() = %q, want unknown", got)
	}
}

// TestCloneCopiesTheNewFieldsAndSharesTokens is Task 4's regression test:
// Clone is a plain struct copy for Object's new scalar fields (nothing to
// deep-copy, unlike the slice fields Clone already walks by hand), and
// Tokens -- set once at genesis and never mutated -- is shared between the
// original and the clone rather than copied.
func TestCloneCopiesTheNewFieldsAndSharesTokens(t *testing.T) {
	g := NewGame([]string{"a", "b"})
	g.Tokens = map[string]*cards.Card{"x": {}}
	o := g.AddObject(nil, 0)
	o.X, o.CastFlags, o.ChosenName, o.ChosenType, o.ChosenNumber, o.AttachedTo, o.IsToken, o.IsCopy = 2, FlagKicked, "n", "t", 4, 7, true, true
	c := g.Clone()
	co := c.Obj(o.ID)
	// Object has slice fields (Counters, Targets, ...), so the whole struct
	// is not comparable with == -- compare the eight new scalar fields by
	// hand instead.
	if co.X != o.X || co.CastFlags != o.CastFlags || co.ChosenName != o.ChosenName ||
		co.ChosenType != o.ChosenType || co.ChosenNumber != o.ChosenNumber ||
		co.AttachedTo != o.AttachedTo || co.IsToken != o.IsToken || co.IsCopy != o.IsCopy {
		t.Fatalf("clone %+v vs %+v", *co, *o)
	}
	if c.Tokens["x"] != g.Tokens["x"] {
		t.Fatal("Tokens is immutable and should be shared, not copied")
	}
	if !co.Ephemeral() || !(&Object{}).Ephemeral() || (&Object{Card: &cards.Card{}}).Ephemeral() {
		t.Fatal("Ephemeral")
	}
}
