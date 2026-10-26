package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 118.12a / 118.3 (cr-g113): an "unless [player] [cost]" cost outside the
// old strict grammar (tapXType, AddCounter, DamageYou<X>, Waterbend, Blight,
// CollectEvidence, ExileFromStack, a folded Draw<X/...>) is offered whenever
// the payer CAN pay it, and paying it settles through the same payer-choice
// continuation as Sac/Discard. Each test drives the real corpus card through
// the kernel's tape and has a pay row and a decline row.

// ugOffer fails unless the pending decision is the unless-pay ask for seat p
// and returns whether a pay option is offered.
func ugOffer(t *testing.T, e *Engine, p state.PlayerID) bool {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "unless_pay" || d.Player != p {
		t.Fatalf("pending = %+v, want the unless-pay ask for seat %d", d, p)
	}
	for _, o := range d.Options {
		if o.Mode == decision.ModeUnlessPay {
			return true
		}
	}
	return false
}

// ugPick answers the pending unless_cost KChoose with the options whose Obj
// is in want (all of them, in offered order, when want is nil).
func ugPick(t *testing.T, e *Engine, kind string, want ...state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "unless_cost" {
		t.Fatalf("pending = %+v, want the unless_cost KChoose", d)
	}
	var idx []int
	for _, o := range d.Options {
		if o.Kind != kind {
			t.Fatalf("option kind = %q, want %q", o.Kind, kind)
		}
		if len(want) == 0 {
			idx = append(idx, o.Index)
			continue
		}
		for _, w := range want {
			if o.Obj == w {
				idx = append(idx, o.Index)
			}
		}
	}
	if len(idx) < d.Min || len(idx) > d.Max {
		t.Fatalf("picked %d of %+v, want %d..%d", len(idx), d.Options, d.Min, d.Max)
	}
	submitChoices(t, e, idx...)
}

func ugCorpus(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	return onBoardCard(t, e, p, mustCorpusCard(t, testutil.CorpusRegistry(t), name))
}

// ugPushTrigger puts the source's first trigger on the stack and resolves it
// as a kernel probe, leaving the unless-pay ask pending.
func ugPushTrigger(e *Engine, src state.ObjID, controller state.PlayerID, targets ...state.ObjID) {
	e.emit(events.Event{Kind: events.TriggerPush, Obj: src, Player: controller, Amount: 0})
	if len(targets) > 0 {
		ability := e.G.Stack[len(e.G.Stack)-1]
		e.emit(events.Event{Kind: events.TargetsChosen, Obj: ability, IDs: targets})
	}
	kr9ResolveTop(e)
}

// ugBridge is Command Bridge as it is when its enters-tapped replacement has
// applied: tapped, so it cannot pay its own tap cost.
func ugBridge(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	bridge := ugCorpus(t, e, 0, "Command Bridge")
	e.emit(events.Event{Kind: events.Tap, Obj: bridge})
	return bridge
}

func ugZone(e *Engine, id state.ObjID) state.Zone { return e.G.Obj(id).Zone }

// Command Bridge: "sacrifice it unless you tap an untapped permanent you
// control" (CR 701.26a: only an untapped permanent can be tapped).
func TestUnlessTapTypeCommandBridge(t *testing.T) {
	t.Parallel()
	t.Run("pay taps a chosen permanent and the Bridge stays", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8101)
		bridge := ugBridge(t, e)
		bear1 := ugCorpus(t, e, 0, "Grizzly Bears")
		bear2 := ugCorpus(t, e, 0, "Grizzly Bears")
		ugPushTrigger(e, bridge, 0)
		if !ugOffer(t, e, 0) {
			t.Fatal("two untapped permanents, yet no pay option")
		}
		answerUnlessPay(t, e, true)
		ugPick(t, e, "tapcost", bear2)
		if !e.G.Obj(bear2).Tapped || e.G.Obj(bear1).Tapped {
			t.Fatalf("tapped = [%v %v], want only the chosen permanent tapped", e.G.Obj(bear1).Tapped, e.G.Obj(bear2).Tapped)
		}
		if z := ugZone(e, bridge); z != state.ZBattlefield {
			t.Fatalf("Bridge zone = %v, want it kept", z)
		}
	})
	t.Run("a lone candidate is tapped without a pick", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8102)
		bridge := ugBridge(t, e)
		bear := ugCorpus(t, e, 0, "Grizzly Bears")
		ugPushTrigger(e, bridge, 0)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, true)
		if !e.G.Obj(bear).Tapped || ugZone(e, bridge) != state.ZBattlefield {
			t.Fatalf("bear tapped=%v bridge zone=%v, want tapped and kept", e.G.Obj(bear).Tapped, ugZone(e, bridge))
		}
	})
	t.Run("decline sacrifices the Bridge", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8103)
		bridge := ugBridge(t, e)
		bear := ugCorpus(t, e, 0, "Grizzly Bears")
		ugPushTrigger(e, bridge, 0)
		ugOffer(t, e, 0)
		answerUnlessPay(t, e, false)
		if z := ugZone(e, bridge); z != state.ZGraveyard {
			t.Fatalf("Bridge zone = %v, want sacrificed", z)
		}
		if e.G.Obj(bear).Tapped {
			t.Fatal("a declined payment tapped a permanent")
		}
	})
	t.Run("no untapped permanent offers only the decline", func(t *testing.T) {
		t.Parallel()
		e := stealEngine(t, 8104)
		bridge := ugBridge(t, e)
		bear := ugCorpus(t, e, 0, "Grizzly Bears")
		e.emit(events.Event{Kind: events.Tap, Obj: bear})
		ugPushTrigger(e, bridge, 0)
		if ugOffer(t, e, 0) {
			t.Fatal("pay offered with every other permanent tapped (CR 118.3, 701.26a)")
		}
		answerUnlessPay(t, e, false)
		if z := ugZone(e, bridge); z != state.ZGraveyard {
			t.Fatalf("Bridge zone = %v, want sacrificed", z)
		}
	})
}
