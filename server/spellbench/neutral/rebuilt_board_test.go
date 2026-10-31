package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestVehicleKeepsItsPrintedPowerAndToughness: the observation gives P/T only
// to a creature, and native's board reads a Vehicle's printed pair.
func TestVehicleKeepsItsPrintedPowerAndToughness(t *testing.T) {
	s := fixtureSession(t, "Name:Fixture Cart\nManaCost:2\nTypes:Artifact Vehicle\nPT:3/4\n")
	obs := seatPriority([]protocol.ObjectRecord{obsPerm("o-cart", "Fixture Cart", "p0", untapped(), "artifact")})
	p := translated(t, s, priorityDecision(obs))
	for _, cv := range p.View.Players[0].Battlefield {
		if cv.Name == "Fixture Cart" {
			if cv.Power != 3 || cv.Toughness != 4 {
				t.Fatalf("vehicle reads %d/%d, want 3/4", cv.Power, cv.Toughness)
			}
			return
		}
	}
	t.Fatal("the vehicle is not on the rebuilt battlefield")
}

// TestShadowPlaysTheObservedBackFace: a double-faced card played as its land
// face is observed under that face's name. The shadow has to show that face,
// or the seat's plans lose the land's mana.
func TestShadowPlaysTheObservedBackFace(t *testing.T) {
	s := fixtureSession(t, fixturePlains,
		"Name:Fixture Hymn\nManaCost:1 W\nTypes:Sorcery\nAlternateMode:Modal\nOracle:x\n"+
			"ALTERNATE\n"+
			"Name:Fixture Mire\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ W | SpellDescription$ Add {W}.\nOracle:x\n")
	obs := seatPriority([]protocol.ObjectRecord{obsPerm("o-mire", "Fixture Mire", "p0", untapped(), "land")})
	sh, err := s.B.shadow(&obs, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range sh.e.Pending().Options {
		if o.Kind == "activate" && sh.ids[o.Obj] == s.B.IDs.Of("o-mire") {
			return
		}
	}
	t.Fatalf("the back face offers no mana: %+v", sh.e.Pending().Options)
}
