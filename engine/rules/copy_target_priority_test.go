package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 117.3b: after a resolution, the active player receives priority with no
// passes counted, also when that resolution asked for a copy's new targets (CR
// 707.10c). Seat 1 answers seat 0's Lightning Bolt with Reverberate.
func TestCopyTargetAskResolutionGivesActivePlayerPriority(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	eng, cfg := miscHandsEngine(t, reg,
		[]string{"Lightning Bolt"}, []string{"Reverberate"}, nil, nil)
	addMana(t, eng, 0, "R")
	addMana(t, eng, 1, "RR")
	bolt := miscHandObj(t, eng, 0, "Lightning Bolt")
	submitChoices(t, eng, miscCastOption(t, eng, bolt))
	answerPlayerTarget := func(p state.PlayerID) {
		t.Helper()
		d := eng.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("expected a target ask, got %+v", d)
		}
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player == p {
				submitChoices(t, eng, o.Index)
				return
			}
		}
		t.Fatalf("seat %d not offered: %+v", p, d.Options)
	}
	answerPlayerTarget(1)
	submitPass(t, eng) // seat 0 passes with the Bolt on the stack
	rev := miscHandObj(t, eng, 1, "Reverberate")
	submitChoices(t, eng, miscCastOption(t, eng, rev))
	if d := eng.Pending(); d != nil && d.Kind == decision.KTarget {
		for _, o := range d.Options {
			if o.Obj == bolt {
				submitChoices(t, eng, o.Index)
				break
			}
		}
	}
	d := passUntilNonPriority(t, eng, 10)
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "copy_targets" {
		t.Fatalf("expected the copy-target ask, got %+v", d)
	}
	answerPlayerTarget(0)
	if len(eng.G.Stack) != 2 || eng.G.Stack[0] != bolt || !eng.G.Obj(eng.G.Stack[1]).IsCopy {
		t.Fatalf("stack %v after the election, want the Bolt (%d) and its copy; pending %+v", eng.G.Stack, bolt, eng.Pending())
	}
	d = eng.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != eng.G.Active || eng.G.Passes != 0 {
		who := -1
		if d != nil {
			who = int(d.Player)
		}
		t.Fatalf("after the election: %+v to seat %d with %d pass(es) counted, want priority to the active seat %d with none (CR 117.3b)",
			d, who, eng.G.Passes, eng.G.Active)
	}
	replayCheck(t, eng, cfg)
}
