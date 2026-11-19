package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEDHETBCloneRidersVesuvanShapeshifter is the end-to-end Vesuvan
// Shapeshifter loop: cast face up as the as-enters copy election (Duration$
// UntilFacedown rides the copy), the granted upkeep trigger turns it face
// down -- which ends the copy -- and the morph turn-up re-offers the copy
// through the printed R:Event$ TurnFaceUp replacement.
func TestEDHETBCloneRidersVesuvanShapeshifter(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Vesuvan Shapeshifter")
	bear := oppBear(t, e)
	id := searchMoveByName(t, e, "Vesuvan Shapeshifter", state.ZHand)
	addMana(t, e, 0, "CCCUU")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision pending: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == "" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no plain cast option for Vesuvan Shapeshifter: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	for i := 0; d != nil && d.Kind == decision.KPriority && i < 10; i++ {
		submitChoices(t, e, 0) // pass priority
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("expected the ETB copy election, got %+v", d)
	}
	if !offersObj(d, bear) {
		t.Fatalf("the election does not offer the Bear: %+v", d.Options)
	}
	submitChoices(t, e, objOptionIndex(t, d, bear))
	passUntilStackEmpty(t, e, 20)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.FaceDown || o.Face() == nil ||
		o.Face().Name != "Grizzly Bears" || o.CopyFace == nil {
		t.Fatalf("Vesuvan did not enter as the Bear copy: %+v", o)
	}
	if len(o.Face().Triggers) == 0 {
		t.Fatalf("the copy carries no granted upkeep trigger: %+v", o.Face().Triggers)
	}

	// Seat 0's next upkeep: the granted upkeep trigger offers to turn the
	// copy face down. Accepting ends the copy (Duration$ UntilFacedown).
	e.setStep(state.StepUpkeep)
	e.putTriggersOnStack()
	for i := 0; i < 12; i++ {
		d = e.Pending()
		if d == nil {
			break
		}
		switch d.Kind {
		case decision.KPriority:
			if len(e.G.Stack) == 0 {
				d = nil
				break
			}
			passed := false
			for _, o := range d.Options {
				if o.Kind == "pass" {
					if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
						t.Fatalf("submit pass: %v", err)
					}
					passed = true
					break
				}
			}
			if !passed {
				t.Fatalf("priority decision with no pass option: %+v", d.Options)
			}
		case decision.KTriggerOrder:
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("submit trigger order: %v", err)
			}
		case decision.KTriggerOptional:
			// Yes: turn the copy face down (the granted VesShapeTurn).
			submitChoices(t, e, d.Options[0].Index)
		default:
			t.Fatalf("unexpected decision during the upkeep drive: %+v", d)
		}
	}
	d = e.Pending()
	d = e.Pending()
	passUntilStackEmpty(t, e, 20)
	o = e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || !o.FaceDown ||
		o.CopyFace != nil || o.Face() == nil || o.Face().Name != "Vesuvan Shapeshifter" {
		t.Fatalf("the turn-down did not end the copy: %+v", o)
	}

	// The morph turn-up (CR 702.36d: the printed face's morph survives the
	// turn-down) re-offers the copy through the printed TurnFaceUp
	// replacement.
	addMana(t, e, 0, "CU")
	idx = turnFaceUpIndex(t, e, id)
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KReplacement || d.Options[0].Kind != "apply" {
		t.Fatalf("the turn-up did not offer the copy replacement: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || !offersObj(d, bear) {
		t.Fatalf("the replacement did not ask which creature to copy: %+v", d)
	}
	submitChoices(t, e, objOptionIndex(t, d, bear))
	passUntilStackEmpty(t, e, 20)
	o = e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.FaceDown || o.Face() == nil ||
		o.Face().Name != "Grizzly Bears" || o.CopyFace == nil {
		t.Fatalf("the turn-up replacement did not re-copy the Bear: %+v", o)
	}
	replayCheck(t, e, cfg)
}
