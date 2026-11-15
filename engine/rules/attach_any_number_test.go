package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Ardenn's "attach any number of Auras and Equipment you control to target
// permanent or player" (an optional attach of any Auras and Equipment you
// control) poses ONE subset ask over the objects with a legal
// destination, and each picked object attaches.

// ardennSubsetBoard builds Ardenn + two Equipment, fires the begin-combat
// trigger, targets Ardenn, and drives to the subset ask.
func ardennSubsetBoard(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID, state.ObjID, *decision.Decision) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := dsBoard(t, reg, "Ardenn, Intrepid Archaeologist", "Bonesplitter", "Short Sword")
	ardenn, a, b := ids["Ardenn, Intrepid Archaeologist"], ids["Bonesplitter"], ids["Short Sword"]
	for _, id := range []state.ObjID{ardenn, a, b} {
		if e.G.Obj(id).Zone != state.ZBattlefield {
			t.Fatalf("precondition: object %d must be on the battlefield", id)
		}
	}
	if a == b {
		t.Fatal("precondition: the two Equipment must be different objects")
	}
	for _, id := range []state.ObjID{a, b} {
		if e.G.Obj(id).AttachedTo != 0 {
			t.Fatalf("precondition: Equipment %d must start unattached", id)
		}
	}
	e.G.Active = 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepBeginCombat})
	for i := 0; i < 3000; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch {
		case d.Kind == decision.KChoose && d.ResumeKind == "attach_subset":
			return e, cfg, ardenn, a, b, d
		case d.Kind == decision.KChoose && d.ResumeKind == "attach_optional":
			t.Fatalf("got the %q ask (%s), want the subset ask", d.Prompt, d.ResumeKind)
		case d.Kind == decision.KPriority:
			submitPass(t, e)
		case d.Kind == decision.KTriggerOptional:
			submitChoices(t, e, 0)
		case d.Kind == decision.KTriggerOrder:
			idx := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				idx = append(idx, o.Index)
			}
			submitChoices(t, e, idx...)
		case d.Kind == decision.KTarget:
			pick := -1
			for _, o := range d.Options {
				if o.Obj == ardenn {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("target ask does not offer Ardenn: %+v", d.Options)
			}
			submitChoices(t, e, pick)
		default:
			t.Fatalf("unexpected decision: %+v", d)
		}
	}
	t.Fatal("the subset ask was never posed")
	return nil, cfg, 0, 0, 0, nil
}

func subsetOptionFor(t *testing.T, d *decision.Decision, obj state.ObjID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == "card" && o.Obj == obj {
			return o.Index
		}
	}
	t.Fatalf("subset ask does not offer object %d: %+v", obj, d.Options)
	return -1
}

func settleAfterSubset(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 200 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision settling: %+v", d)
		}
		submitPass(t, e)
	}
}

func assertSubsetAsk(t *testing.T, d *decision.Decision, a, b state.ObjID) (ia, ib int) {
	t.Helper()
	if d.Min != 0 || d.Max != 2 || len(d.Options) != 2 {
		t.Fatalf("subset ask shape = Min %d Max %d, %d options; want 0/2/2", d.Min, d.Max, len(d.Options))
	}
	ia, ib = subsetOptionFor(t, d, a), subsetOptionFor(t, d, b)
	if ia == ib {
		t.Fatal("precondition: the ask must offer two distinct options")
	}
	return ia, ib
}

func TestArdennAttachesAnyNumber(t *testing.T) {
	t.Parallel()
	e, cfg, ardenn, a, b, d := ardennSubsetBoard(t)
	ia, ib := assertSubsetAsk(t, d, a, b)
	submitChoices(t, e, ia, ib)
	settleAfterSubset(t, e)
	for _, id := range []state.ObjID{a, b} {
		if got := e.G.Obj(id).AttachedTo; got != ardenn {
			t.Fatalf("Equipment %d attached to %d, want Ardenn %d", id, got, ardenn)
		}
	}
	replayCheck(t, e, cfg)
}

func TestArdennAttachSubsetPicksOne(t *testing.T) {
	t.Parallel()
	e, _, ardenn, a, b, d := ardennSubsetBoard(t)
	_, ib := assertSubsetAsk(t, d, a, b)
	submitChoices(t, e, ib)
	settleAfterSubset(t, e)
	if got := e.G.Obj(b).AttachedTo; got != ardenn {
		t.Fatalf("picked Equipment attached to %d, want Ardenn %d", got, ardenn)
	}
	if got := e.G.Obj(a).AttachedTo; got != 0 {
		t.Fatalf("unpicked Equipment attached to %d, want nothing", got)
	}
}

func TestArdennAttachSubsetDeclineAttachesNone(t *testing.T) {
	t.Parallel()
	e, _, _, a, b, d := ardennSubsetBoard(t)
	assertSubsetAsk(t, d, a, b)
	submitChoices(t, e)
	settleAfterSubset(t, e)
	for _, id := range []state.ObjID{a, b} {
		if got := e.G.Obj(id).AttachedTo; got != 0 {
			t.Fatalf("Equipment %d attached to %d after a decline", id, got)
		}
	}
}

// TestArdennAttachSubsetBotAnswerIsLegal runs the default bot's answer to the
// subset ask through the decision validator.
func TestArdennAttachSubsetBotAnswerIsLegal(t *testing.T) {
	t.Parallel()
	e, _, _, a, b, d := ardennSubsetBoard(t)
	assertSubsetAsk(t, d, a, b)
	got := botpolicy.NewGameBot(7).Answer(e.G, e, d)
	if err := d.Validate(got); err != nil {
		t.Fatalf("bot answer %v rejected: %v", got.Choices, err)
	}
	if err := d.Validate(botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player})); err != nil {
		t.Fatalf("clamped empty answer rejected: %v", err)
	}
}
