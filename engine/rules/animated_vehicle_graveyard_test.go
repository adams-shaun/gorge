package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const graveBruteSrc = "Name:Grave Brute\nTypes:Creature\nPT:1/1\nS:Mode$ Continuous | AffectedDefined$ Self | AddPower$ 2 | AddToughness$ 2 | AddAllCreatureTypes$ True | IsPresent$ Creature.YouOwn | PresentCompare$ GE2 | PresentZone$ Graveyard\nOracle:x\n"
const graveVehicleSrc = "Name:Grave Vehicle\nTypes:Artifact Vehicle\nPT:3/3\nOracle:x\n"

// A Vehicle crewed until end of turn is a creature only on the battlefield: the
// object that dies is a new one (CR 400.7) and an artifact card in the
// graveyard, so "two or more creature cards in your graveyard" does not count
// it, in the same event that moved it (the static gate is not stamped from the
// layer-4 table of the head before the move).
func TestAnimatedVehicleThatDiedIsNoCreatureCard(t *testing.T) {
	e, _, _ := newFixtureDeck(t, 954, "Name:Blank\nTypes:Sorcery\nOracle:x\n")
	brute := onBoard(t, e, 0, graveBruteSrc)
	bear := onBoard(t, e, 0, deliriumBearSrc)
	veh := onBoard(t, e, 0, graveVehicleSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.AddContinuous(ContinuousEffect{Source: veh, Layer: LType, Affects: "Card.Self", AddTypes: []string{"Creature"}, UntilEOT: true})
	if got := e.Power(brute); got != 1 {
		t.Fatalf("one creature card in the graveyard: power = %d, want 1", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: veh, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.Power(brute); got != 1 {
		t.Fatalf("one creature card and a Vehicle that was a creature on the battlefield: power = %d, want 1", got)
	}
}
