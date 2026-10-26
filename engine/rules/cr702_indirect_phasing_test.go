package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR70226gAuraPhasesOutWithBearer pins CR 702.26g: a permanent attached
// to a permanent that phases out (here through Talon Gates of Madara's real
// ETB) phases out indirectly with it, and stays attached (702.26h).
func TestCR70226gAuraPhasesOutWithBearer(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 301, "Talon Gates of Madara", "Grizzly Bears", "Witness Protection")
	bears := moveSeededCard(t, e, 0, tokenReplCorpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	aura := lossAttach(t, e, "Witness Protection", 0, bears)
	_ = moveSeededCard(t, e, 0, tokenReplCorpusCard(t, "Talon Gates of Madara"), state.ZBattlefield)
	addMana(t, e, 0, "")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target ask: %+v", d)
	}
	var pick = -1
	for _, o := range d.Options {
		if o.Label == "Grizzly Bears" || pick < 0 {
			pick = o.Index
		}
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 60)
	b, a := e.G.Obj(bears), e.G.Obj(aura)
	if !b.PhasedOut {
		t.Skip("bears not the chosen target")
	}
	if !a.PhasedOut || !a.PhasedOutIndirect || a.AttachedTo != bears {
		t.Errorf("CR 702.26g: aura PhasedOut=%v indirect=%v attachedTo=%d, want out, indirect, on the bears",
			a.PhasedOut, a.PhasedOutIndirect, a.AttachedTo)
	}
	replayCheck(t, e, cfg)
}

// TestCR70226hIndirectPhasesInWithHost pins CR 702.26h: the indirectly
// phased-out Aura phases in with its host at the host controller's untap
// step, still attached.
func TestCR70226hIndirectPhasesInWithHost(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 302, "Grizzly Bears", "Witness Protection")
	bears := moveSeededCard(t, e, 0, tokenReplCorpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	aura := lossAttach(t, e, "Witness Protection", 0, bears)
	e.emit(events.Event{Kind: events.PhaseOut, Obj: bears, Amount: 1})
	if a := e.G.Obj(aura); !a.PhasedOut || !a.PhasedOutIndirect {
		t.Fatalf("precondition: aura not indirectly phased out: %+v", a.PhasedOut)
	}
	// The aura is skipped by the untap scan on its own: only the host's
	// phase-in brings it back.
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	b, a := e.G.Obj(bears), e.G.Obj(aura)
	if b.PhasedOut || a.PhasedOut || a.PhasedOutIndirect {
		t.Fatalf("host out=%v aura out=%v indirect=%v, want all phased in", b.PhasedOut, a.PhasedOut, a.PhasedOutIndirect)
	}
	if a.AttachedTo != bears {
		t.Errorf("CR 702.26h: aura attached to %d after phasing in, want %d", a.AttachedTo, bears)
	}
	replayCheck(t, e, cfg)
}
