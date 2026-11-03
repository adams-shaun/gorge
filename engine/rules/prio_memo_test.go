package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestFaceStepSensitiveTokens(t *testing.T) {
	plain := &cards.Face{Name: "plain", Abilities: []*cards.SA{{Kind: "AB", API: "Draw", Params: map[string]string{"Cost": "T"}}}}
	if faceStepSensitive(plain) {
		t.Fatal("plain ability flagged step-sensitive")
	}
	for name, f := range map[string]*cards.Face{
		"ability phases": {Abilities: []*cards.SA{{Kind: "AB", Params: map[string]string{"ActivationPhases": "Upkeep"}}}},
		"sub chain":      {Abilities: []*cards.SA{{Kind: "AB", Sub: &cards.SA{Kind: "DB", Params: map[string]string{"ConditionPhases": "Main1"}}}}},
		"static phases":  {Statics: []cards.Static{{Mode: "CantBeCast", Params: map[string]string{"Phases": "End of Turn"}}}},
		"svar head":      {SVars: map[string]string{"X": "Count$InOwnMainPhase.1.0"}},
		"first combat":   {Abilities: []*cards.SA{{Kind: "AB", Params: map[string]string{"ActivationFirstCombat": "True"}}}},
		"sneak":          {Keywords: []string{"Sneak:2 R"}},
		"paradigm":       {Keywords: []string{"Paradigm"}},
	} {
		if !faceStepSensitive(f) {
			t.Errorf("%s: not flagged step-sensitive", name)
		}
	}
	// A face that grows after the first read is re-scanned.
	g := &cards.Face{Name: "grows"}
	if faceStepSensitive(g) {
		t.Fatal("empty face flagged")
	}
	g.Statics = append(g.Statics, cards.Static{Mode: "Continuous", Params: map[string]string{"Phases": "Main1"}})
	if !faceStepSensitive(g) {
		t.Fatal("grown face kept its stale answer")
	}
}

// holdOutProbeSrc is a permanent with one free activated ability.
const holdOutProbeSrc = "Name:Hold Out Probe\nTypes:Artifact\n" +
	"A:AB$ GainLife | Cost$ T | LifeAmount$ 1 | SpellDescription$ Gain 1 life.\n" +
	"Oracle:{T}: You gain 1 life.\n"

// An activation the engine is holding out of the offer list (the second
// no-progress abort of the same source, CR 733.2) is withheld by the walk
// through suppressedCast, a runtime field outside the event log, and the next
// plain step change clears it. A list stored while the hold-out stood must not
// be served after it ended: the walk offers the activation again.
func TestPrioMemoSkipsWindowsWhileCastHeldOut(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := quietBase(t, reg)
	id := onBoard(t, e, 0, holdOutProbeSrc)
	offered := func(opts []decision.Option) bool {
		for _, o := range opts {
			if o.Kind == "ability" && o.Obj == id {
				return true
			}
		}
		return false
	}
	if !offered(e.legalActionsWithWindow(0, nil)) {
		t.Fatalf("precondition: the walk does not offer the ability: %v", optSummary(e.legalActionsWithWindow(0, nil)))
	}
	oldVerify := prioMemoVerify
	prioMemoVerify = false
	defer func() { prioMemoVerify = oldVerify }()

	e.suppressedCast = map[state.ObjID]bool{id: true}
	if offered(e.priorityOptions(0, nil)) {
		t.Fatal("precondition: the held-out ability is still offered")
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain2})
	if e.suppressedCast != nil {
		t.Fatal("precondition: the step change did not clear the hold-out")
	}
	if !offered(e.priorityOptions(0, nil)) {
		t.Fatal("the window after the hold-out ended was served a list without the ability the walk offers")
	}
}
