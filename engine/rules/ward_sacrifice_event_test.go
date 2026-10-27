package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// wardSacListenerSrc is a creature whose Sacrificed trigger gains 1 life.
const wardSacListenerSrc = "Name:Sac Listener\nTypes:Creature\nPT:1/1\n" +
	"T:Mode$ Sacrificed | ValidCard$ Creature.YouCtrl | Execute$ TrigGain | TriggerZones$ Battlefield | TriggerDescription$ Whenever you sacrifice a creature, gain 1 life.\n" +
	"SVar:TrigGain:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"

// wardSacOnceListenerSrc is the "one or more" form of the same payoff.
const wardSacOnceListenerSrc = "Name:Sac Once Listener\nTypes:Creature\nPT:1/1\n" +
	"T:Mode$ SacrificedOnce | ValidCard$ Creature.YouCtrl | Execute$ TrigGain | TriggerZones$ Battlefield | TriggerDescription$ Whenever you sacrifice one or more creatures, gain 1 life.\n" +
	"SVar:TrigGain:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"

// wardSacPaid targets a real Vein Ripper (seat 0) with a seat-1 spell and pays
// the ward by sacrificing the plain creature; it returns the engine, the
// sacrificed creature, and the log index where the payment began.
func wardSacPaid(t *testing.T, listener string) (*Engine, state.ObjID, int, int32) {
	t.Helper()
	e := combatEngine(t)
	warded := onBoardCard(t, e, 0, corpusKeywordCard(t, "Vein Ripper"))
	sac := onBoard(t, e, 1, "Name:Payment\nTypes:Creature\nPT:1/1\nOracle:x\n")
	onBoard(t, e, 1, listener)
	cause := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{warded}})
	e.putTriggersOnStack()
	e.resolveTop()
	if err := e.Submit(decision.Intent{Seq: e.Pending().Seq, Player: 1, Choices: []int{0}}); err != nil {
		t.Fatal(err)
	}
	d := e.Pending()
	if d == nil || d.ResumeKind != "ward_sac" {
		t.Fatalf("Vein Ripper ward did not ask for sacrifice: %+v", d)
	}
	if o := e.G.Obj(sac); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: payment creature not on the battlefield: %+v", o)
	}
	// Choices index the offered candidates; pick the plain creature explicitly.
	pick := -1
	for i, op := range d.Options {
		if op.Obj == sac {
			pick = i
		}
	}
	if pick < 0 {
		t.Fatalf("precondition: payment creature %d not among the ward_sac options %+v", sac, d)
	}
	start, life := len(e.L.Events), e.G.Players[1].Life
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{pick}}); err != nil {
		t.Fatal(err)
	}
	if o := e.G.Obj(sac); o.Zone != state.ZGraveyard {
		t.Fatalf("payment creature zone = %s, want graveyard", o.Zone)
	}
	return e, sac, start, life
}

// wardSacSettle answers Vein Ripper's own "whenever a creature dies" target
// ask (the sacrificed creature dies too) so the payer's listener queues, and
// returns the stack once everything is on it.
func wardSacSettle(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 4; i++ {
		e.putTriggersOnStack()
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			return
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWardSacrificePaymentIsASacrificeEvent(t *testing.T) {
	t.Parallel()
	e, sac, start, life := wardSacPaid(t, wardSacListenerSrc)
	found := false
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.MoveZone && ev.Obj == sac && events.IsSacrifice(ev) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no sacrifice MoveZone for the ward payment in %+v", e.L.Events[start:])
	}
	wardSacSettle(t, e)
	for i := 0; i < 8 && len(e.G.Stack) > 1; i++ {
		e.resolveTop()
	}
	// Vein Ripper's own death trigger drains the payer for 2; the listener
	// gains 1 back. Without the sacrifice marker only the drain happens (-2).
	if got := e.G.Players[1].Life - life; got != -1 {
		t.Fatalf("payer life change = %d, want -1 (-2 Vein Ripper drain, +1 Sacrificed listener)", got)
	}
}

func TestWardSacrificePaymentQueuesSacrificedOnceOnce(t *testing.T) {
	t.Parallel()
	e, _, _, _ := wardSacPaid(t, wardSacOnceListenerSrc)
	wardSacSettle(t, e)
	n := 0
	for _, id := range e.G.Stack {
		// Vein Ripper's drain trigger is seat 0's; the listener is seat 1's.
		if o := e.G.Obj(id); o != nil && o.Ability != nil && o.Controller == 1 {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("SacrificedOnce triggers on seat 1's stack side = %d, want 1", n)
	}
}

func TestWardDiscardPaymentTextUnchanged(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	warded := onBoard(t, e, 0, "Name:Discard Warded\nTypes:Creature\nPT:2/2\nK:Ward:Discard<1/Card>\nOracle:x\n")
	cause := e.G.Zone(state.ZLibrary, 1)[0]
	disc := e.G.Zone(state.ZLibrary, 1)[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: disc, From: state.ZLibrary, To: state.ZHand})
	e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{warded}})
	e.putTriggersOnStack()
	e.resolveTop()
	if err := e.Submit(decision.Intent{Seq: e.Pending().Seq, Player: 1, Choices: []int{0}}); err != nil {
		t.Fatal(err)
	}
	d := e.Pending()
	if d == nil || d.ResumeKind != "ward_discard" {
		t.Fatalf("ward did not ask for a discard: %+v", d)
	}
	if o := e.G.Obj(disc); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: discard candidate not in hand: %+v", o)
	}
	start := len(e.L.Events)
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{0}}); err != nil {
		t.Fatal(err)
	}
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.MoveZone && ev.To == state.ZGraveyard && ev.From == state.ZHand {
			if ev.Text != "discarded for ward" {
				t.Fatalf("ward discard text = %q, want %q", ev.Text, "discarded for ward")
			}
			return
		}
	}
	t.Fatalf("no hand->graveyard MoveZone in %+v", e.L.Events[start:])
}
