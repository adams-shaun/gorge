package rules

// ds2: the Defined$ TriggeredTargetLKICopy resolver (engine/effects) reads
// TriggerTarget for a BecomesTarget trigger only when the capture marked the
// event as a targeting one (TriggerStack). This test pins that capture on a
// real card line: Cloud Cover's "return that permanent to its owner's hand"
// must record the TARGETED permanent as TriggerTarget, the targeting spell as
// Remembered (which TriggeredSourceController still reads), and must not set
// the Attached arm's TriggerBearer, so the two arms cannot mask each other.
// Horobi's and Cowardice's lines are the same shape.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestBecomesTargetCaptureMarksTheTargetingEvent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cover := lookup(t, reg, "Cloud Cover")
	bears := lookup(t, reg, "Grizzly Bears")
	zap := lookup(t, reg, "Lightning Bolt")
	e := corpusEngine(t, reg, []*cards.Card{cover, bears}, []*cards.Card{zap})
	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	coverID := moveByName(t, e, 0, "Cloud Cover", state.ZBattlefield)
	spellID := moveByName(t, e, 1, "Lightning Bolt", state.ZStack)

	var trig cards.Trigger
	for _, tr := range e.G.Obj(coverID).Face().Triggers {
		if tr.ModeKind() == cards.TriggerBecomesTarget {
			trig = tr
		}
	}
	if trig.Mode == "" {
		t.Fatal("precondition: Cloud Cover carries no BecomesTarget trigger")
	}
	if v, ok := trig.Param(cards.PKValidTarget); !ok || v == "" {
		t.Fatal("precondition: Cloud Cover's BecomesTarget line has no ValidTarget$")
	}
	// The three roles the resolver and the controller read must be distinct
	// objects, or an assertion below proves nothing.
	if bearID == coverID || bearID == spellID || coverID == spellID {
		t.Fatalf("precondition: distinct ids (bear %d, cover %d, spell %d)", bearID, coverID, spellID)
	}

	ev := events.Event{Kind: events.TargetsChosen, Obj: spellID, Player: 1, IDs: []state.ObjID{bearID}}
	got := e.triggerReferents(trig, coverID, ev, nil)

	if got.TriggerTarget != (state.Target{Obj: bearID}) {
		t.Fatalf("TriggerTarget = %+v, want the targeted permanent %d (source is the enchantment %d)",
			got.TriggerTarget, bearID, coverID)
	}
	if got.TriggerStack != spellID {
		t.Fatalf("TriggerStack = %d, want the targeting spell %d -- the marker the LKI-copy resolver reads", got.TriggerStack, spellID)
	}
	if got.TriggerBearer != 0 {
		t.Fatalf("TriggerBearer = %d, want 0: only an Attached capture sets the bearer arm", got.TriggerBearer)
	}
}
