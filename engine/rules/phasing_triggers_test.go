package rules

// Mode$ PhaseIn / Mode$ PhaseOut (CR 702.26): "whenever ~ phases in" and
// "whenever ~ phases out" triggers. The phased-out look-back (a phased-out
// permanent's abilities do not function, 702.26b, except its own "whenever ~
// phases out") and the indirect case (702.26g: an attachment phases out with
// its host and its own phase-out trigger fires) are the two non-obvious
// halves.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestShimmeringEfreetPhaseInTriggers: Shimmering Efreet phases out, phases in
// at its controller's untap step, and its "whenever ~ phases in, target
// creature phases out" trigger puts the chosen creature out.
func TestShimmeringEfreetPhaseInTriggers(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 311, "Shimmering Efreet", "Grizzly Bears")
	bears := moveSeededCard(t, e, 0, corpusAlternativeCard(t, "Grizzly Bears"), state.ZBattlefield)
	efreet := moveSeededCard(t, e, 0, corpusAlternativeCard(t, "Shimmering Efreet"), state.ZBattlefield)
	e.pending = nil
	if o := e.G.Obj(bears); o == nil || o.PhasedOut || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: bears not a phased-in permanent: %+v", o)
	}
	e.emit(events.Event{Kind: events.PhaseOut, Obj: efreet, Amount: 1})
	if o := e.G.Obj(efreet); !o.PhasedOut {
		t.Fatal("precondition: the Efreet did not phase out")
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("a phase-OUT queued %d trigger(s) for a phase-in-only card", len(e.pendingTriggers))
	}
	// The untap step phases it in; its trigger asks for a target.
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	if o := e.G.Obj(efreet); o.PhasedOut {
		t.Fatal("precondition: the Efreet did not phase in at the untap step")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target ask for the phase-in trigger: %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == bears {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("the bears are not among the target options: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 60)
	if o := e.G.Obj(bears); !o.PhasedOut {
		t.Error("the phase-in trigger did not phase the targeted creature out")
	}
	replayCheck(t, e, cfg)
}

// TestErtaisFamiliarPhaseOutTriggers: the Familiar's "when ~ phases out, mill
// three cards" fires from the phased-out status (the look-back of CR
// 702.26b), once, and not for the other permanent's phasing.
func TestErtaisFamiliarPhaseOutTriggers(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 312, "Ertai's Familiar", "Grizzly Bears")
	toMain1(t, e)
	familiar := moveByName(t, e, 0, "Ertai's Familiar", state.ZBattlefield)
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	e.pending = nil
	// Another permanent phasing out is not the Familiar phasing out.
	e.emit(events.Event{Kind: events.PhaseOut, Obj: bears, Amount: 1})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("the bears phasing out queued %d trigger(s) on the Familiar", len(e.pendingTriggers))
	}
	lib := len(e.G.Zone(state.ZLibrary, 0))
	gy := len(e.G.Zone(state.ZGraveyard, 0))
	e.emit(events.Event{Kind: events.PhaseOut, Obj: familiar, Amount: 1})
	if o := e.G.Obj(familiar); !o.PhasedOut || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the Familiar is not a phased-out battlefield permanent: %+v", o)
	}
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("the Familiar's phase-out queued %d trigger(s), want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if got := lib - len(e.G.Zone(state.ZLibrary, 0)); got != 3 {
		t.Errorf("library shrank by %d, want 3", got)
	}
	if got := len(e.G.Zone(state.ZGraveyard, 0)) - gy; got != 3 {
		t.Errorf("graveyard grew by %d, want 3", got)
	}
	replayCheck(t, e, cfg)
}

// TestIndirectPhaseOutTriggers: an Aura with a phase-out trigger phases out
// indirectly with its host (CR 702.26g) and its trigger fires; the host's
// own phase-out event is the only one emitted.
func TestIndirectPhaseOutTriggers(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	host := onBoard(t, e, 0, "Name:Host\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	aura := onBoard(t, e, 0, "Name:Ward Aura\nManaCost:1 W\nTypes:Enchantment Aura\nK:Enchant creature\n"+
		"T:Mode$ PhaseOut | ValidCard$ Card.phasedOutSelf | Execute$ TrigLife | TriggerDescription$ Whenever this Aura phases out, you gain 3 life.\n"+
		"SVar:TrigLife:DB$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n")
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{host}})
	if got := e.G.Obj(aura).AttachedTo; got != host {
		t.Fatalf("precondition: aura attached to %d, want %d", got, host)
	}
	e.pending = nil
	before := e.G.Players[0].Life
	n := len(e.L.Events)
	e.emit(events.Event{Kind: events.PhaseOut, Obj: host, Amount: 1})
	a := e.G.Obj(aura)
	if !a.PhasedOut || !a.PhasedOutIndirect {
		t.Fatalf("precondition: aura PhasedOut=%v indirect=%v, want out indirectly", a.PhasedOut, a.PhasedOutIndirect)
	}
	phaseEvents := 0
	for _, ev := range e.L.Events[n:] {
		if ev.Kind == events.PhaseOut {
			phaseEvents++
		}
	}
	if phaseEvents != 1 {
		t.Fatalf("precondition: %d PhaseOut events logged, want only the host's", phaseEvents)
	}
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("the aura's indirect phase-out queued %d trigger(s), want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if got := e.G.Players[0].Life - before; got != 3 {
		t.Errorf("life changed by %d, want +3 from the aura's trigger", got)
	}
}

// TestPhaseOutAllStillBatches: The War Doctor's batch trigger still fires once
// for one Guardian of Faith resolution that phases out two permanents (the new
// per-permanent modes must not disturb the batch latch), and a phased-out
// PhaseOutAll source stays silent (only its own phase-out look-back functions,
// CR 702.26b).
func TestPhaseOutAllStillBatches(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 601, "The War Doctor", "Guardian of Faith", "Grizzly Bears", "Grizzly Bears")
	toMain1(t, e)
	doctor := moveByName(t, e, 0, "The War Doctor", state.ZBattlefield)
	b1 := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	b2 := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	moveByName(t, e, 0, "Guardian of Faith", state.ZBattlefield)
	addMana(t, e, 0, "")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Guardian of Faith's target ask, got %+v", d)
	}
	var picks []int
	for _, o := range d.Options {
		if o.Obj == b1 || o.Obj == b2 {
			picks = append(picks, o.Index)
		}
	}
	if len(picks) != 2 {
		t.Fatalf("precondition: the ask offered %d of the two bears", len(picks))
	}
	submitChoices(t, e, picks...)
	passUntilStackEmpty(t, e, 60)
	if !e.G.Obj(b1).PhasedOut || !e.G.Obj(b2).PhasedOut {
		t.Fatal("precondition: the bears did not both phase out")
	}
	e.putTriggersOnStack()
	if len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	if got := e.G.Obj(doctor).Counter("TIME"); got != 1 {
		t.Fatalf("PhaseOutAll fired %d times for one two-permanent batch, want exactly 1", got)
	}
	// The Doctor itself phasing out: its PhaseOutAll line does not function.
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.PhaseOut, Obj: doctor, Amount: 1})
	if !e.G.Obj(doctor).PhasedOut {
		t.Fatal("precondition: the Doctor did not phase out")
	}
	if len(e.pendingTriggers) != 0 {
		t.Errorf("a phased-out PhaseOutAll source queued %d trigger(s) (CR 702.26b)", len(e.pendingTriggers))
	}
	replayCheck(t, e, cfg)
}

// TestEveryPhasingTriggerCardIsPlayable is the class census: the five cards
// whose only unsupported primitives were trig:PhaseIn / trig:PhaseOut.
func TestEveryPhasingTriggerCardIsPlayable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{
		"King of the Oathbreakers", "Shimmering Efreet", "Warping Wurm", "Teferi's Imp", "Ertai's Familiar",
	} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("corpus card %q missing", name)
		}
		if miss := reg.Unsupported(c, sup); len(miss) != 0 {
			t.Errorf("%s is still unsupported: %v", name, miss)
		}
	}
}
