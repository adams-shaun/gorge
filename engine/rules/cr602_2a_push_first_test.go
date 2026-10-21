package rules

// CR 602.2a: "To propose the activation of an ability, the player first
// moves that ability from where it is to the stack ... It becomes the topmost
// object on the stack." Then 602.2b imports 601.2b-i: the targets, X and
// costs are chosen with the ability already on the stack. Real corpus cards
// read as script text; the board is built eventlessly and funded through the
// pool.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestCR602_2aAbilityIsOnTheStackBeforeItsTarget: Prodigal Pyromancer's
// "{T}: deal 1 damage to any target". When the target is asked, the ability
// is already the topmost stack object, sourced by the Pyromancer; answering
// the target taps the Pyromancer and leaves exactly that object on the stack.
func TestCR602_2aAbilityIsOnTheStackBeforeItsTarget(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	pyro := battlePerm(t, e, 0, corpusCardText(t, "p/prodigal_pyromancer.txt"))
	e.askPriority(0)
	cr601Choose(t, e, "activate", func(o decision.Option) bool { return o.Kind == "ability" && o.Obj == pyro })
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("want the Pyromancer's target ask, got %+v", d)
	}
	n := len(e.G.Stack)
	if n != 1 {
		t.Fatalf("CR 602.2a: stack %v while the target is chosen, want the activated ability on it", e.G.Stack)
	}
	ab := e.G.Obj(e.G.Stack[n-1])
	if ab == nil || ab.Ability == nil || ab.Source != pyro {
		t.Fatalf("CR 602.2a: top of stack %+v is not the Pyromancer's ability", ab)
	}
	cr601Choose(t, e, "target", func(o decision.Option) bool { return o.Kind == "player" && o.Label == "b" })
	if !e.G.Obj(pyro).Tapped {
		t.Error("the {T} cost was not paid")
	}
	if len(e.G.Stack) != 1 || e.G.Stack[0] != ab.ID {
		t.Errorf("stack %v after the activation, want only ability %d", e.G.Stack, ab.ID)
	}
	if len(ab.Targets) != 1 || !ab.Targets[0].IsPlayer || ab.Targets[0].Player != 1 {
		t.Errorf("ability targets %+v, want player b", ab.Targets)
	}
}

// TestCR602_2aAbortedActivationKeepsItsOncePerTurnUse: Walking Wall's
// "{3}: ... Activate only once each turn" proposed with an empty pool. The
// activation is reversed (CR 733.1) after its object was pushed (CR 602.2a);
// the reversal must undo the activation census, so the ability has not been
// "activated this turn": it is offered again once {3} is available, and the
// Wall's ActivatedThisTurn is back to 0.
func TestCR602_2aAbortedActivationKeepsItsOncePerTurnUse(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	wall := battlePerm(t, e, 0, corpusCardText(t, "w/walking_wall.txt"))
	e.askPriority(0)
	e.pending = nil // a direct proposal, as cr733's activation_mana site makes
	e.beginActivation(0, decision.Option{Kind: "ability", Obj: wall, Ability: 0})
	if e.cast != nil {
		t.Fatalf("the unpayable activation did not abort: pending %+v", e.Pending())
	}
	aborted := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Text == "activation aborted: cost no longer payable" {
			aborted = true
		}
	}
	if !aborted {
		t.Fatal("no activation abort Note: the probe did not reach the CR 733.1 path")
	}
	if got := e.G.Obj(wall).ActivatedThisTurn; got != 0 {
		t.Errorf("Walking Wall ActivatedThisTurn = %d after the reversed activation, want 0", got)
	}
	if used := e.activationUsedCount(wall, 0, "", true); used != 0 {
		t.Errorf("activationUsedCount = %d after the reversed activation, want 0", used)
	}
	if n, ok := e.AbilitiesActivatedThisTurnMatching(0, 0, nil, nil, "Activated.YouCtrl"); n != 0 || !ok {
		t.Errorf("AbilitiesActivatedThisTurnMatching = (%d, %v) after the reversed activation, want (0, true)", n, ok)
	}
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: 3})
	e.pending = nil
	e.askPriority(0)
	offered := false
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == wall {
			offered = true
		}
	}
	if !offered {
		t.Error("Walking Wall's once-each-turn ability is not offered after a reversed activation (CR 733.1)")
	}
	if len(e.G.Stack) != 0 {
		t.Errorf("stack not empty after the reversal: %v", e.G.Stack)
	}
}
