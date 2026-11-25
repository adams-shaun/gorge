package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedExaltedTriggerLabel pins the label of a layer-6 granted Exalted
// (xp xpt-9efa4021, Merchant of Truth): the queue entry has no face trigger
// line, so the order ask labelled both Clue triggers with the bare token
// name, which no driver can bind to a trigger. They read "<name>: exalted".
func TestGrantedExaltedTriggerLabel(t *testing.T) {
	t.Parallel()
	merchant := mshCorpusCardPath(t, "Merchant of Truth", "m/merchant_of_truth.txt")
	e := threeSeatEngine(t)
	onBoardCard(t, e, 0, merchant)
	clueSrc := "Name:Clue Token\nTypes:Artifact Clue\nOracle:x\n"
	c1 := onBoardCard(t, e, 0, card(t, clueSrc))
	c2 := onBoardCard(t, e, 0, card(t, clueSrc))
	bear := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	for _, id := range []state.ObjID{c1, c2} {
		if e.G.Obj(id).Face().HasKeyword("Exalted") || !e.HasKeyword(id, "Exalted") {
			t.Fatalf("precondition: Clue %d must hold Exalted only through the grant", id)
		}
	}
	e.askAttackers()
	bi := batchAttackerOptionIndex(t, e, bear, 1)
	declareAttackersAt(t, e, [][2]int{{int(bear), bi}})
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder || len(d.Options) != 2 {
		t.Fatalf("expected a 2-option trigger-order ask, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Label != "Clue Token: exalted" {
			t.Fatalf("option label %q, want %q (all: %v)", o.Label, "Clue Token: exalted", d.Options)
		}
	}
}

// TestGrantedMeleeTriggerLabel pins the sibling Melee arm of the label helper:
// a layer-6 granted Melee instance (melee.go) has no face trigger line either,
// so its queue entry must label by keyword too. Two granting sources give one
// Bear two Melee instances (CR 702.121b), whose order ask a client binds by
// the "<name>: melee" label.
func TestGrantedMeleeTriggerLabel(t *testing.T) {
	t.Parallel()
	titania := mshCorpusCardPath(t, "Titania, Proud Pummeler", "t/titania_proud_pummeler.txt")
	adriana := mshCorpusCardPath(t, "Adriana, Captain of the Guard", "a/adriana_captain_of_the_guard.txt")
	e := threeSeatEngine(t)
	for _, c := range []*cards.Card{titania, adriana} {
		if id := onBoardCard(t, e, 0, c); e.G.Obj(id).SummonSick {
			e.G.Obj(id).SummonSick = false
		}
	}
	bear := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	instances := 0
	for _, k := range e.Derived(bear).Keywords {
		if cards.KeywordHead(k) == "Melee" {
			instances++
		}
	}
	if e.G.Obj(bear).Face().HasKeyword("Melee") || instances != 2 {
		t.Fatalf("precondition: Bear must hold Melee only through the two grants, got %d instances", instances)
	}
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	bi := batchAttackerOptionIndex(t, e, bear, 1)
	declareAttackersAt(t, e, [][2]int{{int(bear), bi}})
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder || len(d.Options) != 2 {
		t.Fatalf("expected a 2-option trigger-order ask, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Label != "Bear: melee" {
			t.Fatalf("option label %q, want %q (all: %v)", o.Label, "Bear: melee", d.Options)
		}
	}
}
