package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestUntilUntapsAnimateEndsBeforeTheCreatureSacrificePart pins the Tomb of
// Annihilation Oubliette shape ("sacrifice an artifact, a creature, and a
// land", part by part) over a Duration$ UntilUntaps animation: the player
// sacrifices The Blackstaff of Waterdeep for the artifact part, which ends the
// animation of the artifact it made a creature, so that artifact is no
// creature for the creature part.
func TestUntilUntapsAnimateEndsBeforeTheCreatureSacrificePart(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	staff := onBoard(t, e, 0, "Name:Test Staff\nTypes:Legendary Artifact\nA:AB$ Animate | Cost$ 1 U T | ValidTgts$ Artifact.Other+!token+YouCtrl | Duration$ UntilUntaps | Power$ 4 | Toughness$ 4 | Types$ Creature\nOracle:x\n")
	schem := onBoard(t, e, 0, "Name:Test Schematic\nTypes:Artifact\nOracle:x\n")
	bear := onBoard(t, e, 0, "Name:Test Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	bear2 := onBoard(t, e, 0, "Name:Test Other Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	onBoard(t, e, 0, "Name:Test Isle\nTypes:Land\nOracle:x\n")
	tomb := onBoard(t, e, 0, "Name:Test Tomb\nTypes:Artifact\nA:AB$ Sacrifice | Cost$ 0 | Defined$ You | SacValid$ Artifact & Creature & Land | SacEachValid$ True\nOracle:x\n")
	e.emit(events.Event{Kind: events.Tap, Obj: staff})
	face := e.G.Obj(staff).Face()
	effects.Resolve(e, &effects.Ctx{Source: staff, Controller: 0, SVars: face.SVars,
		Targets: []state.Target{{Obj: schem}}}, face.Abilities[0])
	if !e.IsCreature(schem) {
		t.Fatal("precondition: the animated artifact is not a creature")
	}
	e.G.Obj(tomb).SummonSick = false
	activateAbilityOn(t, e, tomb)
	var d *decision.Decision
	for i := 0; i < 20; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("precondition: artifact part asks no choice: %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == staff {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("precondition: the staff is not offered for the artifact part: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
		t.Fatal(err)
	}
	if e.G.Obj(staff).Zone == state.ZBattlefield {
		t.Fatal("precondition: the staff was not sacrificed")
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("creature part asks no choice: %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == schem {
			t.Fatalf("the artifact offered as a creature after its animator left: %+v", d.Options)
		}
	}
	found := 0
	for _, o := range d.Options {
		if o.Obj == bear || o.Obj == bear2 {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("precondition: both bears are offered, got %d: %+v", found, d.Options)
	}
}
