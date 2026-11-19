package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// gainedFaceObjs lists the object ids carrier's GainsAbilitiesOf$ static
// currently grants it faces from.
func gainedFaceObjs(e *Engine, carrier state.ObjID) []state.ObjID {
	var out []state.ObjID
	for _, ce := range e.active() {
		if ce.Source != carrier || ce.Layer != LAbilities {
			continue
		}
		for _, gf := range ce.GainedFaces {
			out = append(out, gf.Obj)
		}
	}
	return out
}

// TestGainsAbilitiesOfSkipsFaceDownSource pins CR 708.2a: a face-down
// permanent has no text, so a GainsAbilitiesOf$ static grants nothing from it
// (Marvin, Murderous Mimic reading a manifested Dissection Tools), and grants
// from it again once it is turned face up.
func TestGainsAbilitiesOfSkipsFaceDownSource(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	carrier := onBoard(t, e, 0, "Name:Gains Carrier\nTypes:Creature\nPT:1/1\n"+
		"S:Mode$ Continuous | Affected$ Card.Self | GainsAbilitiesOf$ Creature.YouCtrl+!sharesNameWith Self | Description$ x\n"+
		"Oracle:x\n")
	src := onBoard(t, e, 0, "Name:Gains Source\nTypes:Artifact Creature\nPT:1/1\nA:AB$ Draw | Cost$ T | NumCards$ 1\nOracle:x\n")

	if got := gainedFaceObjs(e, carrier); len(got) != 1 || got[0] != src {
		t.Fatalf("precondition: face-up grant = %v, want [%d]", got, src)
	}
	e.G.Obj(src).FaceDown = true
	e.staticEpoch, e.activeEpoch = -1, -1
	if o := e.G.Obj(src); !o.FaceDownHidden() {
		t.Fatalf("precondition: source %+v is not face down on the battlefield", o)
	}
	if got := gainedFaceObjs(e, carrier); len(got) != 0 {
		t.Fatalf("grant from a face-down source = %v, want none", got)
	}
}
