package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A world forked from a world (the az seat's search worlds are redeals of
// the honest root, itself a redeal of the live engine) at a posed tape
// resolution re-runs from the REAL S0. The outer world's redeal events,
// appended after its own fork, must be re-applied by the inner world's
// re-run as well as the inner world's own -- else the re-run diverges at the
// first of them (the azmcts "re-executed event differs: recorded
// hidden_identity / library_order" panics, ~4% of all az simulations).
func TestKernelNestedHypotheticalRedealWorld(t *testing.T) {
	for _, innerRedeal := range []bool{false, true} {
		e, _ := kernelFixture(t, 2, 9901, tapeDrawVoteSrc,
			"Name:Tape Filler A\nManaCost:1\nTypes:Artifact\nOracle:x\n",
			"Name:Tape Filler B\nManaCost:2\nTypes:Artifact\nOracle:x\n",
			"Name:Tape Filler C\nManaCost:3\nTypes:Artifact\nOracle:x\n")
		moveByName(t, e, 0, "Tape Filler A", state.ZLibrary)
		moveByName(t, e, 0, "Tape Filler B", state.ZLibrary)
		moveByName(t, e, 0, "Tape Filler C", state.ZLibrary)
		addMana(t, e, 0, "B")
		submitChoices(t, e, castOptionFor(t, e, fixtureInHand(t, e, "Tape Draw Vote")).Index)
		for i := 0; i < 20; i++ {
			d := e.Pending()
			if d.Kind == decision.KChoose && d.Player == 1 {
				break
			}
			if d.Kind == decision.KPriority {
				submitChoices(t, e, tapePassIndex(d))
				continue
			}
			submitChoices(t, e, 0)
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Player != 1 || !TapePosed(e) {
			t.Fatalf("precondition: seat 1's vote not posed on a tape resolution: %+v", d)
		}
		root := e.CloneHypothetical(0xfeed)
		tapeTestRedeal(root, 0, 77)
		w := root.CloneHypothetical(0xbeef)
		if innerRedeal {
			tapeTestRedeal(w, 0, 78)
		}
		wantHand := append([]state.ObjID(nil), w.G.Zone(state.ZHand, 0)...)
		wantTop := w.G.Zone(state.ZLibrary, 0)[0]
		var err error
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("inner redeal %v: nested world re-run panicked: %v", innerRedeal, p)
				}
			}()
			err = w.SubmitHypothetical(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{1}})
		}()
		if err != nil {
			t.Fatalf("inner redeal %v: world submit: %v", innerRedeal, err)
		}
		gotHand := w.G.Zone(state.ZHand, 0)
		if len(gotHand) != len(wantHand)+1 || gotHand[len(gotHand)-1] != wantTop {
			t.Fatalf("inner redeal %v: post-fork draw read %v, want the world's top %d", innerRedeal, gotHand, wantTop)
		}
		// The outer world still re-runs on its own.
		if err := root.SubmitHypothetical(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{1}}); err != nil {
			t.Fatalf("outer world submit: %v", err)
		}
	}
}
