package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestChooseSourceNoCandidateDoesNotWedge pins the zero-candidate shape: a
// ChooseSource whose Choices$ filter matches nothing poses no decision (the
// empty-answer-only KChoose is absorbed by effects.Ask), records no chosen
// source, and the follow-up replacement therefore never fires — damage flows
// through untouched rather than the seat being stranded on an ask it cannot
// answer.
func TestChooseSourceNoCandidateDoesNotWedge(t *testing.T) {
	reg := sharedCorpus(t)
	e := newSeats(t, 2)
	e.pending = nil
	// Only a green creature exists, so Card.BlueSource matches nothing — not
	// even the (white/red) Deflecting Palm itself.
	onBoard(t, e, 1, "Name:Elf\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n")
	palm := e.G.AddObject(mustCorpusCard(t, reg, "Deflecting Palm"), 0)
	palm.Zone = state.ZStack
	sa := palm.Face().SpellAbility()
	redSpec := *sa
	redSpec.Params = map[string]string{}
	for k, v := range sa.Params {
		redSpec.Params[k] = v
	}
	redSpec.Params["Choices"] = "Card.BlueSource"
	effects.Resolve(e, &effects.Ctx{Source: palm.ID, Controller: 0, SVars: palm.Face().SVars}, &redSpec)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("zero-candidate ChooseSource posed a decision: %+v", d)
	}
	if len(palm.Chosen) != 0 {
		t.Fatalf("chosen = %+v, want nothing", palm.Chosen)
	}
}
