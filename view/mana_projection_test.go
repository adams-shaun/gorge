package view

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
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

func TestCardViewProjectsNonManaAbilityCostsOnlyWhereASeatCanAct(t *testing.T) {
	g := state.NewGame([]string{"alice", "bob"})
	gadget := parsedWithIntrinsics(t, "gadget.txt", "Name:Gadget\nManaCost:2\nTypes:Artifact\nA:AB$ Draw | Cost$ 2 T | Oracle:x\nA:AB$ Mana | Cost$ T | Produced$ C | Oracle:x\nA:AB$ Destroy | Cost$ Sac<1/Artifact> | Oracle:x\n")
	bf := g.AddObject(gadget, 0)
	hand := g.AddObject(gadget, 0)
	grave := g.AddObject(gadget, 0)
	hand.Zone = state.ZHand
	grave.Zone = state.ZGraveyard
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{bf.ID})
	g.SetZone(state.ZHand, 0, []state.ObjID{hand.ID})
	g.SetZone(state.ZGraveyard, 0, []state.ObjID{grave.ID})

	pv := Project(g, flatChars{g}, 0, nil).Players[0]
	want := []string{"2 T", "Sac<1/Artifact>"}
	if got := pv.Battlefield[0].AbilityCosts; !slices.Equal(got, want) {
		t.Fatalf("battlefield ability costs = %#v, want %#v", got, want)
	}
	if got := pv.Hand[0].AbilityCosts; !slices.Equal(got, want) {
		t.Fatalf("own hand ability costs = %#v, want %#v", got, want)
	}
	if got := pv.Graveyard[0].AbilityCosts; got != nil {
		t.Fatalf("graveyard ability costs = %#v, want nil", got)
	}

	// A real rules.Engine supplies effective rather than merely printed costs.
	// This is the reviewer's live shape: one generic reduction makes a printed
	// {2}, {T} activation payable after one offered colourless tap.
	discount := parsedWithIntrinsics(t, "discount.txt", "Name:Discount\nManaCost:2\nTypes:Artifact\nS:Mode$ ReduceCost | ValidCard$ Artifact.YouCtrl | Type$ Ability | Amount$ 1\nOracle:x\n")
	eg := state.NewGame([]string{"alice", "bob"})
	ability := eg.AddObject(gadget, 0)
	reducer := eg.AddObject(discount, 0)
	ability.Zone = state.ZBattlefield
	reducer.Zone = state.ZBattlefield
	eg.SetZone(state.ZBattlefield, 0, []state.ObjID{ability.ID, reducer.ID})
	e := &rules.Engine{G: eg, L: events.NewLog(1)}
	effective := Project(eg, e, 0, nil).Players[0].Battlefield[0].AbilityCosts
	if wantEffective := []string{"1 T", "Sac<1/Artifact>"}; !slices.Equal(effective, wantEffective) {
		t.Fatalf("effective battlefield ability costs = %#v, want %#v", effective, wantEffective)
	}
}

// TestCardViewProjectsAnyAsItsRealAlternatives is the honesty requirement at
// the view layer, restated for the resolution-time colour choice: an "add any
// colour" source is projected as the five colours it can really be tapped for
// plus the Any flag, never as the colourless stand-in the executor used to
// emit before it asked. The Any flag is what says the five slots are
// ALTERNATIVES -- one unit, of a colour chosen on activation (CR 106.1b) --
// so a client and the policy can aim a coloured pip at this source while
// still knowing its colour is not fixed. The colourless slot must stay empty:
// the pool never receives a colourless from this source any more, and a
// phantom colourless there is exactly the claim the old approximation made.
func TestCardViewProjectsAnyAsItsRealAlternatives(t *testing.T) {
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
		if !cv.Produces.ProducesColour(i) {
			t.Errorf("Cavern must offer colour %d as a real alternative", i)
		}
	}
	if cv.Produces.Colour[5] != 0 {
		t.Errorf("Cavern colourless = %d, want 0 (the executor asks for a colour, it no longer emits colourless)", cv.Produces.Colour[5])
	}
	// The five slots are one unit each: the projection must not inflate the
	// source into five mana. A plain basic beside it pins the unit scale.
	plains := parsedWithIntrinsics(t, "plains.txt",
		"Name:Plains\nManaCost:no cost\nTypes:Basic Land Plains\nOracle:x\n")
	pid := g.AddObject(plains, 0).ID
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{oid, pid})
	both := Project(g, flatChars{g}, 0, nil).Players[0].Battlefield
	for i := 0; i < 5; i++ {
		if got, want := both[0].Produces.Colour[i], both[1].Produces.Colour[0]; got != want {
			t.Errorf("Cavern colour %d = %d, want %d (one unit, the same scale as a Plains' white)", i, got, want)
		}
	}
}
