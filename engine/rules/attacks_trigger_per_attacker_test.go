package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 508.3a: "Whenever [a creature] attacks" triggers if that creature is
// declared as an attacker, and CR 603.2c lets one event with several
// occurrences trigger the ability repeatedly. A Mode$ Attacks line with a
// non-self ValidCard$ therefore fires once PER matching attacker; before
// this fix the engine queued one trigger per DeclareAttackers event and
// Defined$ TriggeredAttacker fell back to every remembered attacker.
func TestAttacksTriggerPerMatchingAttacker(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	kragma := onBoardCard(t, e, 0, corpusCard(t, "Kragma Warcaller"))
	min1 := onBoardReady(t, e, 0, "Name:Minotaur One\nManaCost:2 R\nTypes:Creature Minotaur\nPT:2/2\nOracle:x\n")
	min2 := onBoardReady(t, e, 0, "Name:Minotaur Two\nManaCost:2 R\nTypes:Creature Minotaur\nPT:2/2\nOracle:x\n")
	ogre := onBoardReady(t, e, 0, "Name:Ogre\nManaCost:2 R\nTypes:Creature Ogre\nPT:2/2\nOracle:x\n")
	// Precondition: Kragma is the trigger source (not an attacker) and the
	// declaration holds exactly the two Minotaurs plus one non-Minotaur.
	if got := e.G.Obj(kragma).Face().Triggers[0].ParamStr(cards.PKValidCard); got != "Minotaur.YouCtrl" {
		t.Fatalf("precondition: Kragma ValidCard$ = %q, want Minotaur.YouCtrl", got)
	}
	if e.Power(min1) != 2 || e.Power(min2) != 2 || e.Power(ogre) != 2 {
		t.Fatalf("precondition: attackers start at %d/%d/%d, want 2/2/2", e.Power(min1), e.Power(min2), e.Power(ogre))
	}

	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{min1, min2, ogre}})

	// One trigger per MATCHING attacker, not one per event and not one per
	// declared attacker (the Ogre matches nothing).
	if got := len(e.pendingTriggers); got != 2 {
		t.Fatalf("Kragma queued %d triggers for two attacking Minotaurs, want 2", got)
	}
	e.putTriggersOnStack()
	if d := e.Pending(); d == nil || d.Kind != decision.KTriggerOrder || len(d.Options) != 2 {
		t.Fatalf("per-attacker triggers must be ordered as separate instances, got %+v", d)
	} else {
		submitChoices(t, e, 0, 1)
	}
	drainTriggers(t, e)

	if got := e.Power(min1); got != 4 {
		t.Errorf("first Minotaur power = %d, want 4 (+2/+0)", got)
	}
	if got := e.Power(min2); got != 4 {
		t.Errorf("second Minotaur power = %d, want 4 (+2/+0)", got)
	}
	if got := e.Power(ogre); got != 2 {
		t.Errorf("non-Minotaur power = %d, want 2 (no pump)", got)
	}
}

// TestZoralineGainsPerBat pins the same rule on a body whose Defined$ is a
// player, not the attacker: "Whenever a Bat you control attacks, you gain 1
// life" must gain 1 per attacking Bat, not 1 per DeclareAttackers event.
func TestZoralineGainsPerBat(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	onBoardCard(t, e, 0, corpusCard(t, "Zoraline, Cosmos Caller"))
	bats := []state.ObjID{
		onBoardReady(t, e, 0, "Name:Bat One\nManaCost:1 B\nTypes:Creature Bat\nPT:1/1\nOracle:x\n"),
		onBoardReady(t, e, 0, "Name:Bat Two\nManaCost:1 B\nTypes:Creature Bat\nPT:1/1\nOracle:x\n"),
		onBoardReady(t, e, 0, "Name:Bat Three\nManaCost:1 B\nTypes:Creature Bat\nPT:1/1\nOracle:x\n"),
	}
	// Precondition: three distinct attacking Bats, so one event carries three
	// occurrences of the triggering condition.
	if len(bats) != 3 || bats[0] == bats[1] || bats[1] == bats[2] || bats[0] == bats[2] {
		t.Fatalf("precondition: want three distinct Bats, got %v", bats)
	}
	lifeBefore := e.G.Players[0].Life

	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: bats})

	if got := len(e.pendingTriggers); got != 3 {
		t.Fatalf("Zoraline queued %d triggers for three attacking Bats, want 3", got)
	}
	drainTriggers(t, e)

	if got := e.G.Players[0].Life; got != lifeBefore+3 {
		t.Fatalf("Zoraline controller life = %d, want %d (1 per attacking Bat)", got, lifeBefore+3)
	}
}

// TestAttackersDeclaredStillBatches is the control: the BATCH "whenever one
// or more creatures attack" mode (Mode$ AttackersDeclared) keeps its
// one-per-declaration latch and does not split per attacker.
func TestAttackersDeclaredStillBatches(t *testing.T) {
	t.Parallel()
	bard := mshCorpusCardPath(t, "Bard, Heir of Girion", "b/bard_heir_of_girion.txt")
	e := threeSeatEngine(t)
	onBoardCard(t, e, 0, bard)
	a := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b := onBoardReady(t, e, 0, "Name:Fox\nManaCost:1 W\nTypes:Creature Fox\nPT:2/2\nOracle:x\n")
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	handBefore := len(e.G.Zone(state.ZHand, 0))

	e.askAttackers()
	ai := batchAttackerOptionIndex(t, e, a, 1)
	bi := batchAttackerOptionIndex(t, e, b, 1)
	declareAttackersAt(t, e, [][2]int{{int(a), ai}, {int(b), bi}})

	// Precondition: both attackers really landed at one defender.
	nAt1 := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.DeclareAttackers && ev.Player == 1 {
			nAt1 += len(ev.IDs)
		}
	}
	if nAt1 != 2 {
		t.Fatalf("precondition: seat 1 was attacked by %d attackers, want 2", nAt1)
	}
	if got := batchTriggerPushes(e); got != 1 {
		t.Fatalf("batch trigger pushed %d times for one declaration, want 1", got)
	}
	e.resolveTop()
	if got := len(e.G.Zone(state.ZHand, 0)) - handBefore; got != 1 {
		t.Fatalf("batch trigger drew %d cards for one declaration, want 1", got)
	}
}
