package view

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// parsedWithIntrinsics parses a one-card script, links it and applies the
// intrinsic layer the way corpus load does, so a basic land's subtype-granted
// mana ability is present when the view projects it.
func parsedWithIntrinsics(t *testing.T, name, src string) *cards.Card {
	t.Helper()
	c, err := cards.ParseBytes(name, []byte(src))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// TestCardViewProjectsManaProduction pins the new CardView.Produces field: a
// basic Plains projects one white (its intrinsic), a dual Tundra projects
// white and blue (both intrinsic halves), and a colourless rock projects two
// colourless. A card with no mana ability projects a nil Produces, so the
// wire is not bloated by a six-entry array on every permanent.
func TestCardViewProjectsManaProduction(t *testing.T) {
	g := state.NewGame([]string{"alice", "bob"})
	plains := parsedWithIntrinsics(t, "plains.txt",
		"Name:Plains\nManaCost:no cost\nTypes:Basic Land Plains\nOracle:x\n")
	tundra := parsedWithIntrinsics(t, "tundra.txt",
		"Name:Tundra\nManaCost:no cost\nTypes:Land Plains Island\nOracle:x\n")
	rock := parsedWithIntrinsics(t, "rock.txt",
		"Name:Rock\nManaCost:2\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | Oracle:x\n")
	bear := parsedWithIntrinsics(t, "bear.txt",
		"Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	oids := []state.ObjID{
		g.AddObject(plains, 0).ID,
		g.AddObject(tundra, 0).ID,
		g.AddObject(rock, 0).ID,
		g.AddObject(bear, 0).ID,
	}
	g.SetZone(state.ZBattlefield, 0, oids)

	cvs := Project(g, flatChars{g}, 0, nil).Players[0].Battlefield
	if cvs[0].Produces == nil || !cvs[0].Produces.ProducesColour(0) || cvs[0].Produces.Colour[0] != 1 {
		t.Fatalf("Plains produces = %v, want one white", cvs[0].Produces)
	}
	if cvs[1].Produces == nil || !cvs[1].Produces.ProducesColour(0) || !cvs[1].Produces.ProducesColour(1) {
		t.Fatalf("Tundra produces = %v, want white and blue", cvs[1].Produces)
	}
	if cvs[2].Produces == nil || cvs[2].Produces.Colour[5] != 2 {
		t.Fatalf("rock produces = %v, want two colourless", cvs[2].Produces)
	}
	// A creature with no mana ability stays nil so the wire stays small.
	if cvs[3].Produces != nil {
		t.Fatalf("Bear produces = %v, want nil (no mana ability)", cvs[3].Produces)
	}
}

// TestCardViewProjectsAnyConservatively is the honesty requirement at the
// view layer: an "add any colour" source is projected as the colourless the
// engine actually resolves plus the Any flag -- never a coloured pip the pool
// will not receive -- so a client and the policy both see that this source
// cannot be leaned on for a specific colour.
func TestCardViewProjectsAnyConservatively(t *testing.T) {
	g := state.NewGame([]string{"alice", "bob"})
	cavern := parsedWithIntrinsics(t, "cavern.txt",
		"Name:Cavern\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Any | Oracle:x\n")
	oid := g.AddObject(cavern, 0).ID
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{oid})

	cv := Project(g, flatChars{g}, 0, nil).Players[0].Battlefield[0]
	if cv.Produces == nil || !cv.Produces.Any {
		t.Fatalf("Cavern produces = %v, want the Any/conditional flag set", cv.Produces)
	}
	for i := 0; i < 5; i++ {
		if cv.Produces.ProducesColour(i) {
			t.Errorf("Cavern must not assert a coloured pip %d", i)
		}
	}
	if cv.Produces.Colour[5] != 1 {
		t.Errorf("Cavern colourless = %d, want 1 (the colourless the engine resolves)", cv.Produces.Colour[5])
	}
}
