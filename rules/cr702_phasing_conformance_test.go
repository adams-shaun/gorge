package rules

// Reference: Magic: The Gathering Comprehensive Rules, 2026-08-07.
// CR 702.25: Phasing (702.25a–f); 702.25b/d phased-out status.
// CR 502.4: the untap step's turn-based action phases permanents in.
// These are engine-boundary probes on real corpus cards, never fabricated
// rules text. Every leaf passes with the fix, so all run in the ordinary
// lane; `make conformance` runs them via -run TestCR.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// cr702corpusCard looks a real corpus card up by name.
func cr702corpusCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	return tokenReplCorpusCard(t, name)
}

// TestCR702PhasedOutPermanentIsNotATarget pins CR 702.25b: a phased-out
// permanent is treated as though it does not exist, so the target census
// never offers it. The probe reads the same candidatesFor the target ask
// uses; a phased-in control creature IS offered, so the exclusion is real,
// not a vacuous empty list.
func TestCR702PhasedOutPermanentIsNotATarget(t *testing.T) {
	e, cfg := phasesGame(t, 401, "Grizzly Bears")
	bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	sa := &cards.SA{Params: map[string]string{"ValidTgts": "Creature"}}
	// Precondition: while phased in, the creature IS offered (the census has
	// a real candidate to lose).
	if got := e.legalTargetCandidates(0, 0, 0, sa); !cr702hasObj(got, bears) {
		t.Fatal("precondition: a phased-in creature is not offered as a Creature target")
	}
	e.emit(events.Event{Kind: events.PhaseOut, Obj: bears, Amount: 1})
	if o := e.G.Obj(bears); o == nil || !o.PhasedOut {
		t.Fatal("precondition: Bears are not phased out")
	}
	if got := e.legalTargetCandidates(0, 0, 0, sa); cr702hasObj(got, bears) {
		t.Fatal("CR 702.25b: a phased-out permanent was offered as a target")
	}
	replayCheck(t, e, cfg)
}

// TestCR702PhasedOutPermanentStaticAbilityIsOff pins CR 702.25b: a phased-out
// permanent's static abilities are off, so a lord's pump does not reach it.
// Glorious Anthem is the lord; the Bears' effective power is the probe.
func TestCR702PhasedOutPermanentStaticAbilityIsOff(t *testing.T) {
	e, cfg := phasesGame(t, 402, "Glorious Anthem", "Grizzly Bears")
	anthem := moveSeededCard(t, e, 0, cr702corpusCard(t, "Glorious Anthem"), state.ZBattlefield)
	bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	if o := e.G.Obj(anthem); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: Glorious Anthem is not on the battlefield")
	}
	// Precondition: the pump applies while the creature is phased in.
	if got := e.Power(bears); got != 3 {
		t.Fatalf("precondition: Grizzly Bears power = %d, want 3 (2/2 + Glorious Anthem)", got)
	}
	e.emit(events.Event{Kind: events.PhaseOut, Obj: bears, Amount: 1})
	if got := e.Power(bears); got != 2 {
		t.Fatalf("CR 702.25b: phased-out Bears power = %d, want 2 (the lord's static must be off)", got)
	}
	replayCheck(t, e, cfg)
}

// TestCR702PhasedOutPermanentDoesNotAttack pins CR 702.25b: a phased-out
// permanent does not participate in combat. canAttack is the one gate both
// the attacker offer and the validator read. Raging Goblin carries Haste, so
// the only gate under test is phasing (no summoning-sickness setup needed).
func TestCR702PhasedOutPermanentDoesNotAttack(t *testing.T) {
	e, cfg := phasesGame(t, 403, "Raging Goblin")
	goblin := moveSeededCard(t, e, 0, cr702corpusCard(t, "Raging Goblin"), state.ZBattlefield)
	if o := e.G.Obj(goblin); o == nil || o.Zone != state.ZBattlefield || !e.HasKeyword(goblin, "Haste") {
		t.Fatal("precondition: Raging Goblin is not a hasty battlefield creature")
	}
	if !e.canAttack(goblin) {
		t.Fatal("precondition: a phased-in, hasty creature cannot attack")
	}
	e.emit(events.Event{Kind: events.PhaseOut, Obj: goblin, Amount: 1})
	if e.canAttack(goblin) {
		t.Fatal("CR 702.25b: a phased-out creature was allowed to attack")
	}
	replayCheck(t, e, cfg)
}

// TestCR702PhasedOutPermanentPhasesInAtUntap pins CR 702.25d with CR 502.4:
// a phased-out permanent phases in at its controller's untap step.
func TestCR702PhasedOutPermanentPhasesInAtUntap(t *testing.T) {
	e, cfg := phasesGame(t, 404, "Grizzly Bears")
	bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	e.emit(events.Event{Kind: events.PhaseOut, Obj: bears, Amount: 1})
	if o := e.G.Obj(bears); o == nil || !o.PhasedOut {
		t.Fatal("precondition: Bears are not phased out")
	}
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	if o := e.G.Obj(bears); o == nil || o.PhasedOut {
		t.Fatal("CR 702.25d: a phased-out permanent did not phase in at its controller's untap step")
	}
	replayCheck(t, e, cfg)
}

// cr702hasObj reports whether any candidate names the object.
func cr702hasObj(cands []targetCandidate, id state.ObjID) bool {
	for _, c := range cands {
		if c.obj == id {
			return true
		}
	}
	return false
}
