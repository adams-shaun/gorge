package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGrantedPoisonousTriggerLabelAndMandatory pins the ctx-keyword arm of
// triggerOf/triggerLabel (xp xpt-ade264d5, Brood Sliver): a poisonous trigger
// queued by grantedPoisonousTriggers carries no face Triggers index, so
// triggerOf must not read the source's printed line 0 for it -- on Brood
// Sliver that line is the optional token trigger
// (OptionalDecider$ TriggeredSourceController), which mislabelled the
// mandatory poisonous trigger (CR 702.70a, 603.5: poisonous has no "may") as
// the printed one and hid it behind Brood Sliver's description. Both slivers
// attack an open player; each queues a poisonous 1, and both trigger-order
// options must carry a "poisonous N" label and pose no optional ask.
func TestGrantedPoisonousTriggerLabelAndMandatory(t *testing.T) {
	t.Parallel()
	e, cfg := combatTriggerBoard(t, testutil.CorpusRegistry(t), []string{"Brood Sliver", "Virulent Sliver"}, nil, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	brood := findBattlefield(t, e, 0, "Brood Sliver", 0)
	virulent := findBattlefield(t, e, 0, "Virulent Sliver", 0)
	// Preconditions: Brood's first printed T: line is the optional token
	// trigger -- the line a broken triggerOf would hand the poisonous entry
	// back -- and neither sliver prints Poisonous (both hold it only as a
	// Virulent Sliver grant).
	bs := e.G.Obj(brood).Face()
	if len(bs.Triggers) == 0 || bs.Triggers[0].ParamStr(cards.PKOptionalDecider) == "" {
		t.Fatalf("precondition: Brood Sliver's first T: line is not optional (%d lines)", len(bs.Triggers))
	}
	for _, id := range []state.ObjID{brood, virulent} {
		if e.G.Obj(id).Face().HasKeyword("Poisonous") {
			t.Fatalf("precondition: %s prints Poisonous; not the granted path", e.G.Obj(id).Face().Name)
		}
		if !e.HasKeyword(id, "Poisonous") {
			t.Fatalf("precondition: %s lacks the granted Poisonous", e.G.Obj(id).Face().Name)
		}
	}
	e.askAttackers()
	submitAttackers(t, e, virulent, brood)
	if d := e.Pending(); d != nil && d.Kind == decision.KBlockers {
		submitBlockersOnly(t, e)
		drainCombatPriority(t, e)
	}
	drainCombatDamageToStack(t, e)
	// The order ask: four simultaneous seat-0 triggers -- Brood's printed
	// token trigger once per damaging Sliver (two of them) plus one poisonous
	// per sliver.
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder || len(d.Options) != 4 {
		t.Fatalf("expected a 4-option trigger-order ask, got %+v", d)
	}
	// Labels: exactly one poisonous option per sliver, distinguishable from
	// each other and from the printed line's description.
	seen := map[string]int{}
	for _, o := range d.Options {
		if strings.Contains(o.Label, "poisonous") {
			seen[o.Label]++
		}
	}
	if len(seen) != 2 {
		t.Fatalf("trigger-order options carry %d distinct poisonous labels (%v), want 2", len(seen), d.Options)
	}
	for _, want := range []string{"Brood Sliver: poisonous 1", "Virulent Sliver: poisonous 1"} {
		if seen[want] != 1 {
			t.Fatalf("poisonous label %q bound %d times in %v", want, seen[want], d.Options)
		}
	}
	// Optionality: a poisonous trigger is mandatory (CR 702.70a); the queue
	// view must not pose Brood's printed OptionalDecider$ for it.
	for _, p := range e.PendingTriggers() {
		if strings.Contains(p.Label, "poisonous") && p.Optional {
			t.Fatalf("poisonous trigger %q posed optional for seat %d", p.Label, p.Decider)
		}
	}
	choices := make([]int, len(d.Options))
	for i, o := range d.Options {
		choices[i] = o.Index
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatal(err)
	}
	// Drain: answer Brood's printed "may" (declined, to keep the board), pass
	// priority, and assert no optional ask ever carries a poisonous label.
	for i := 0; i < 64 && !e.G.Over && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the stack (depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KTriggerOptional {
			if strings.Contains(d.Prompt, "poisonous") {
				t.Fatalf("optional ask posed for a poisonous trigger: %q", d.Prompt)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[1].Index}}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected %s decision while draining: %+v", d.Kind, d)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack still holds %d objects after the drain", len(e.G.Stack))
	}
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("precondition: defender life = %d, want 16 (an unblocked 3/3 and 1/1)", got)
	}
	if got := e.G.Players[1].Counter("POISON"); got != 2 {
		t.Fatalf("defender poison = %d, want 2 (poisonous 1 from each Sliver)", got)
	}
	replayCheck(t, e, cfg)
}
