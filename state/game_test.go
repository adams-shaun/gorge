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
