package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestUntapStunProvenance(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Stunned Creature\nTypes:Creature\nPT:1/1\nOracle:x\n")
	o := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.Tap, Obj: o.ID})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "STUN", Amount: 1})
	if got := h.g.Obj(o.ID); got.Zone != state.ZBattlefield || !got.Tapped || got.Counter("STUN") != 1 {
		t.Fatalf("fixture: zone/tapped/STUN = %v/%v/%d", got.Zone, got.Tapped, got.Counter("STUN"))
	}

	TryUntap(h, o.ID)
	var marked events.Event
	for _, ev := range h.log {
		if ev.Kind == events.CounterChange && ev.Obj == o.ID && ev.Amount < 0 {
			marked = ev
		}
	}
	if marked.Text != events.UntapReplacedByStunNotice {
		t.Fatalf("TryUntap event Text = %q, want marker", marked.Text)
	}

	h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "STUN", Amount: 1})
	c := &Ctx{Source: o.ID, Controller: 0}
	effRemoveCounter(h, c, sa(t, "DB$ RemoveCounter | Defined$ Self | CounterType$ STUN | CounterNum$ 1"))
	var direct events.Event
	for _, ev := range h.log {
		if ev.Kind == events.CounterChange && ev.Obj == o.ID && ev.Amount < 0 && ev.Text == "" {
			direct = ev
		}
	}
	if direct.Kind != events.CounterChange || direct.Counter != "STUN" {
		t.Fatalf("RemoveCounter did not emit unmarked STUN decrement: %+v", direct)
	}
}
