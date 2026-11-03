package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 603.3d: "that creature" in a BecomesTarget trigger's body is the object
// that became a target, not the trigger's source. Pawpatch Recruit watches
// creatures you control, so its TriggerTarget must be the targeted bear.
func TestBecomesTargetReferentIsTheTargetedObject(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	recruit := lookup(t, reg, "Pawpatch Recruit")
	bears := lookup(t, reg, "Grizzly Bears")
	e := corpusEngine(t, reg, []*cards.Card{recruit, bears}, []*cards.Card{})
	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	recruitID := moveByName(t, e, 0, "Pawpatch Recruit", state.ZBattlefield)
	if bearID == recruitID {
		t.Fatal("precondition: bear and recruit are the same object")
	}
	var trig cards.Trigger
	for _, tr := range e.G.Obj(recruitID).Face().Triggers {
		if tr.ModeKind() == cards.TriggerBecomesTarget {
			trig = tr
		}
	}
	if _, ok := trig.Param(cards.PKValidTarget); !ok {
		t.Fatal("precondition: Pawpatch Recruit's BecomesTarget line has no ValidTarget$")
	}
	ev := events.Event{Kind: events.TargetsChosen, Obj: bearID, IDs: []state.ObjID{bearID}}
	got := e.triggerReferents(trig, recruitID, ev, nil)
	if got.TriggerTarget != (state.Target{Obj: bearID}) {
		t.Fatalf("TriggerTarget = %+v, want the targeted bear %d (source is %d)", got.TriggerTarget, bearID, recruitID)
	}
	// A line with no ValidTarget$ keeps the source as its referent.
	self := e.triggerReferents(cards.Trigger{Mode: "BecomesTarget"}, recruitID, ev, nil)
	if self.TriggerTarget != (state.Target{Obj: recruitID}) {
		t.Fatalf("no-ValidTarget$ TriggerTarget = %+v, want source %d", self.TriggerTarget, recruitID)
	}
}

// nonSelfBecomesTargetLines is the corpus count of BecomesTarget(Once)
// triggers whose ValidTarget$ is not purely Self-shaped: exactly the lines
// whose TriggerTarget role must be the targeted object (becomesTargetReferent).
// A new such card must be exercised by the referent; the count only moves
// with a corpus refresh, and a move should be inspected, not just re-pinned.
const nonSelfBecomesTargetLines = 48

func TestBecomesTargetNonSelfCarriersCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	n := 0
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, tr := range f.Triggers {
				if k := tr.ModeKind(); k != cards.TriggerBecomesTarget && k != cards.TriggerBecomesTargetOnce {
					continue
				}
				v, ok := tr.Param(cards.PKValidTarget)
				if !ok || strings.HasSuffix(v, ".Self") && !strings.Contains(v, ",") {
					continue
				}
				n++
			}
		}
	}
	if n != nonSelfBecomesTargetLines {
		t.Fatalf("non-Self BecomesTarget(Once) ValidTarget$ lines = %d, pinned %d", n, nonSelfBecomesTargetLines)
	}
}
